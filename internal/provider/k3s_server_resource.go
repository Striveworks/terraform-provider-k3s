package provider

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int32default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/objectplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"gopkg.in/yaml.v2"
	"striveworks.us/terraform-provider-k3s/internal/k3s"
	"striveworks.us/terraform-provider-k3s/internal/ssh_client"
)

// Ensure structs are properly implements interfaces

var _ resource.ResourceWithConfigValidators = &K3sServerResource{}
var _ resource.ResourceWithConfigure = &K3sServerResource{}
var _ resource.ResourceWithImportState = &K3sServerResource{}

type K3sServerResource struct {
	version *string
}

func NewK3sServerResource() resource.Resource {
	return &K3sServerResource{}
}

func (s *ServerClientModel) sshClient(ctx context.Context) (ssh_client.SSHClient, error) {
	port := 22
	if int(s.Port.ValueInt32()) != 0 {
		port = int(s.Port.ValueInt32())
	}

	return ssh_client.NewSSHClient(ctx, s.Host.ValueString(), port, s.User.ValueString(), s.PrivateKey.ValueString(), s.Password.ValueString())
}

type HaConfig struct {
	ClusterInit types.Bool   `tfsdk:"cluster_init"`
	Token       types.String `tfsdk:"token"`
	Server      types.String `tfsdk:"server"`
	KubeConfig  types.String `tfsdk:"kubeconfig"`
}

func (m HaConfig) AttributeTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"cluster_init": types.BoolType,
		"token":        types.StringType,
		"server":       types.StringType,
		"kubeconfig":   types.StringType,
	}
}

type OidcConfig struct {
	Audience     types.String `tfsdk:"audience"`
	SigningPKCS8 types.String `tfsdk:"pkcs8"`
	SigningKey   types.String `tfsdk:"signing_key"`
	Issuer       types.String `tfsdk:"issuer"`
	JWKSKeys     types.String `tfsdk:"jwks_keys"`
}

func (m OidcConfig) AttributeTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"audience":    types.StringType,
		"pkcs8":       types.StringType,
		"signing_key": types.StringType,
		"issuer":      types.StringType,
		"jwks_keys":   types.StringType,
	}
}

// ServerClientModel describes the resource data model.
type ServerClientModel struct {
	NodeAuth
	// Connection
	Host types.String `tfsdk:"host"`
	Port types.Int32  `tfsdk:"port"`
	// Configs
	BinDir      types.String `tfsdk:"bin_dir"`
	K3sConfig   types.String `tfsdk:"config"`
	K3sRegistry types.String `tfsdk:"registry"`
	// Highly Available config
	HaConfig types.Object `tfsdk:"highly_available"`
	// OIDC Support
	OidcConfig types.Object `tfsdk:"oidc"`

	// Outputs
	Id         types.String `tfsdk:"id"`
	Server     types.String `tfsdk:"server"`
	KubeConfig types.String `tfsdk:"kubeconfig"`
	Token      types.String `tfsdk:"token"`
	Active     types.Bool   `tfsdk:"active"`
}

// Schema implements resource.ResourceWithImportState.
func (s *K3sServerResource) Schema(context context.Context, resource resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: ("Creates a k3s server resource. Only one of `password` or `private_key` can be passed.\n" +
			"If ran in highly available mode, it is up to the consumers of this module to correctly implement " +
			"the raft protocol and create an odd number of ha nodes."),
		Attributes: map[string]schema.Attribute{
			// Inputs
			"bin_dir": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Value of a path used to put the k3s binary",
				Default:             stringdefault.StaticString("/usr/local/bin"),
				Computed:            true,
			},
			"private_key": schema.StringAttribute{
				Optional:            true,
				Sensitive:           true,
				MarkdownDescription: "Private ssh key value to be used in place of a password",
			},
			"password": schema.StringAttribute{
				Optional:            true,
				Sensitive:           true,
				MarkdownDescription: "Username of the target server",
			},
			"user": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Username of the target server",
			},
			// Connection
			"host": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Hostname of the target server",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"port": schema.Int32Attribute{
				Optional:            true,
				Computed:            true,
				Default:             int32default.StaticInt32(22),
				MarkdownDescription: "Override default SSH port (22)",
			},
			// Config
			"config": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "K3s server config",
			},
			"registry": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "K3s server registry",
			},
			// Outputs
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Id of the k3s server resource",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"kubeconfig": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "KubeConfig for the cluster",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"token": schema.StringAttribute{
				Computed:            true,
				Sensitive:           true,
				MarkdownDescription: "Server token used for joining nodes to the cluster",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"server": schema.StringAttribute{
				Computed: true,
				// Optional:            false,
				MarkdownDescription: "Server url  used for joining nodes to the cluster.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"active": schema.BoolAttribute{
				Computed:            true,
				MarkdownDescription: "The health of the server",
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.UseStateForUnknown(),
				},
			},
			"highly_available": schema.SingleNestedAttribute{
				Optional:    true,
				Description: "Run server node in highly available mode",
				PlanModifiers: []planmodifier.Object{
					objectplanmodifier.UseStateForUnknown(),
				},
				Attributes: map[string]schema.Attribute{
					"cluster_init": schema.BoolAttribute{
						Computed:            true,
						Optional:            true,
						Default:             booldefault.StaticBool(false),
						MarkdownDescription: "Node is the init node for the HA cluster",
					},
					"server": schema.StringAttribute{
						Optional:            true,
						MarkdownDescription: "Url of init node",
					},
					"token": schema.StringAttribute{
						Optional:            true,
						Sensitive:           true,
						MarkdownDescription: "Server token used for joining nodes to the cluster",
					},
					"kubeconfig": schema.StringAttribute{
						Optional:            true,
						Sensitive:           true,
						MarkdownDescription: "KubeConfig for the cluster",
					},
				},
			},
			"oidc": schema.SingleNestedAttribute{
				Optional:    true,
				Description: "Support for including oidc provider in k3s",
				Attributes: map[string]schema.Attribute{
					"audience": schema.StringAttribute{
						Required:            true,
						Sensitive:           true,
						MarkdownDescription: "OIDC Audience",
					},
					"pkcs8": schema.StringAttribute{
						Required:            true,
						Sensitive:           true,
						MarkdownDescription: "Public signing key",
					},
					"signing_key": schema.StringAttribute{
						Required:            true,
						Sensitive:           true,
						MarkdownDescription: "Private signing key",
					},
					"issuer": schema.StringAttribute{
						Required:            true,
						Sensitive:           true,
						MarkdownDescription: "Issuer url",
					},
					"jwks_keys": schema.StringAttribute{
						Computed:            true,
						Sensitive:           true,
						MarkdownDescription: "Issuer url",
						PlanModifiers: []planmodifier.String{
							stringplanmodifier.UseStateForUnknown(),
						},
					},
				},
			},
		},
	}
}

// Configure implements resource.ResourceWithConfigure.
func (s *K3sServerResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	provider, ok := req.ProviderData.(*K3sProvider)
	if !ok {
		resp.Diagnostics.AddError("Could not convert provider data into version", "")
		return
	}
	if provider.Version != "" {
		s.version = &provider.Version
	}
}

// Create implements resource.ResourceWithImportState.
func (s *K3sServerResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data ServerClientModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	tflog.Debug(ctx, "Retrieving state")

	if resp.Diagnostics.HasError() {
		return
	}

	sshClient, err := data.sshClient(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Creating ssh config", err.Error())
		return
	}
	tflog.Debug(ctx, "SSH client configured")

	config, err := ParseYamlString(data.K3sConfig)
	if err != nil {
		resp.Diagnostics.AddError("Creating k3s config", err.Error())
		return
	}
	tflog.Debug(ctx, "K3s config parsed")

	registry, err := ParseYamlString(data.K3sRegistry)
	if err != nil {
		resp.Diagnostics.AddError("Creating k3s registry", err.Error())
		return
	}
	config["embedded-registry"] = registry != nil
	tflog.Debug(ctx, "K3s registry parsed")

	// Oidc Support?
	var oidc OidcConfig
	if !data.OidcConfig.IsNull() {
		tflog.Debug(ctx, "See OIDC support requested, adding to config")
		resp.Diagnostics.Append(data.OidcConfig.As(ctx, &oidc, basetypes.ObjectAsOptions{})...)

		config["kube-apiserver-arg"] = []string{
			fmt.Sprintf("api-audiences=%s", oidc.Audience.ValueString()),
			"service-account-key-file=/etc/rancher/k3s/tls/sa-signer-pkcs8.pub",
			"service-account-key-file=/var/lib/rancher/k3s/server/tls/service.key",
			"service-account-signing-key-file=/etc/rancher/k3s/tls/sa-signer.key",
			fmt.Sprintf("service-account-issuer=%s", oidc.Issuer.ValueString()),
			"service-account-issuer=k3s",
		}
	}

	// Determine if we are running a HA server or
	// we are running a single node server
	var server k3s.K3sServer
	if !data.HaConfig.IsNull() {
		var haConfig HaConfig
		resp.Diagnostics.Append(data.HaConfig.As(ctx, &haConfig, basetypes.ObjectAsOptions{})...)
		if haConfig.ClusterInit.ValueBool() {
			tflog.Info(ctx, "Running in node HA init mode")
			config["cluster-init"] = true
			server = k3s.NewK3sServerHAComponent(ctx, config, registry, s.version, "", data.BinDir.ValueString())
		} else {
			tflog.Info(ctx, "Running in node HA join mode")
			config["server"] = haConfig.Server.ValueString()
			server = k3s.NewK3sServerHAComponent(ctx, config, registry, s.version, haConfig.Token.ValueString(), data.BinDir.ValueString())
		}
	} else {
		server = k3s.NewK3sServerComponent(ctx, config, registry, s.version, data.BinDir.ValueString())
	}

	if !data.OidcConfig.IsNull() {
		server.AddFile("/etc/rancher/k3s/tls/sa-signer-pkcs8.pub", oidc.SigningPKCS8.ValueString())
		server.AddFile("/etc/rancher/k3s/tls/sa-signer.key", oidc.SigningKey.ValueString())
	}

	tflog.Info(ctx, "Running k3s server preq steps")
	if err := server.RunPreReqs(sshClient); err != nil {
		resp.Diagnostics.AddError("Running k3s server prereqs", err.Error())
		return
	}

	tflog.Info(ctx, "Running k3s server install")
	if err := server.RunInstall(sshClient); err != nil {
		resp.Diagnostics.AddError("Running k3s server prereqs", err.Error())
		return
	}

	tflog.Info(ctx, "Checking k3s systemd status")
	active, err := server.Status(sshClient)
	if err != nil {
		resp.Diagnostics.AddError("Error retrieving server status", err.Error())
		return
	}

	if !active {
		status, err := server.StatusLog(sshClient)
		if err != nil {
			resp.Diagnostics.AddError("Error retrieving systemctl status", err.Error())
			return
		}
		resp.Diagnostics.AddError("Error running k3s systemctl status", status)

		logs, err := server.Journal(sshClient)
		if err != nil {
			resp.Diagnostics.AddError("Error retrieving journalctl status", err.Error())
			return
		}
		tflog.Trace(ctx, logs)
	}

	// Set outputs
	tflog.Info(ctx, "Setting k3s server outputs")
	data.Active = types.BoolValue(active)
	data.KubeConfig = types.StringValue(server.KubeConfig())
	data.Token = types.StringValue(server.Token())
	data.Id = types.StringValue(fmt.Sprintf("server,%s", data.Host))
	data.Server = types.StringValue(fmt.Sprintf("https://%s:6443", data.Host.ValueString()))
	// Set jwks output
	if !data.OidcConfig.IsNull() {
		jwks, err := server.JWKS(sshClient)
		if err != nil {
			resp.Diagnostics.AddError("Getting JWKS key", err.Error())
			return
		}
		oidc.JWKSKeys = types.StringValue(jwks)
		data.OidcConfig, _ = types.ObjectValueFrom(ctx, oidc.AttributeTypes(), oidc)
	}

	tflog.Info(ctx, "Created a k3s server resource")
	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// Delete implements resource.ResourceWithImportState.
func (s *K3sServerResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data ServerClientModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	sshClient, err := data.sshClient(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Creating ssh config", err.Error())
		return
	}

	kubeconfig := ""
	if !data.HaConfig.IsNull() {
		var haConfig HaConfig
		data.HaConfig.As(ctx, &haConfig, basetypes.ObjectAsOptions{})
		kubeconfig = haConfig.KubeConfig.ValueString()
	}

	server := k3s.NewK3sServerComponent(ctx, nil, nil, nil, data.BinDir.ValueString())
	if err := server.RunUninstall(sshClient, kubeconfig); err != nil {
		resp.Diagnostics.AddError("Creating uninstall k3s", err.Error())
		return
	}

}

// Metadata implements resource.ResourceWithImportState.
func (s *K3sServerResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_server"
}

// Read implements resource.ResourceWithImportState.
func (s *K3sServerResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data ServerClientModel

	// Read Terraform state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	sshClient, err := data.sshClient(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Creating ssh config", err.Error())
		return
	}
	tflog.Info(ctx, "Resyncing k3s_server")
	server := k3s.NewK3sServerComponent(ctx, nil, nil, nil, data.BinDir.ValueString())
	if err := server.Resync(sshClient); err != nil {
		resp.Diagnostics.AddError("failed importing: resyncing k3s_server", err.Error())
		return
	}

	tflog.Info(ctx, "Checking k3s systemd status")
	active, err := server.Status(sshClient)
	if err != nil {
		resp.Diagnostics.AddError("failed importing: Error retrieving server status", err.Error())
		return
	}

	data.Active = types.BoolValue(active)
	data.KubeConfig = types.StringValue(server.KubeConfig())
	data.Token = types.StringValue(server.Token())

	if serverConfig := server.Config(); serverConfig != nil {
		config, err := yaml.Marshal(server.Config())
		if err != nil {
			resp.Diagnostics.AddError("failed importing: Error server config", err.Error())
			return
		}
		data.K3sConfig = types.StringValue(string(config))
	}

	if registry := server.Registry(); registry != nil {
		contents, err := yaml.Marshal(registry)
		if err != nil {
			resp.Diagnostics.AddError("failed importing: Error server registry", err.Error())
			return
		}
		if string(contents) != "" {
			data.K3sRegistry = types.StringValue(string(contents))
		}
	}

	resp.Diagnostics.Append(req.State.Set(ctx, &data)...)
}

// Update implements resource.ResourceWithImportState.
func (s *K3sServerResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data ServerClientModel
	var state ServerClientModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	sshClient, err := data.sshClient(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Creating ssh config", err.Error())
		return
	}

	server := k3s.NewK3sServerComponent(ctx, nil, nil, nil, data.BinDir.ValueString())

	if err := server.Update(sshClient); err != nil {
		resp.Diagnostics.AddError("Error updating server", err.Error())
	}

	tflog.Info(ctx, "Getting k3s server status")
	active, err := server.Status(sshClient)
	if err != nil {
		resp.Diagnostics.AddError("Error retrieving server status", err.Error())
		return
	}

	tflog.Info(ctx, "Setting k3s server outputs")
	state.Active = types.BoolValue(active)
	state.K3sConfig = data.K3sConfig
	state.K3sRegistry = data.K3sRegistry
	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// ConfigValidators implements resource.ResourceWithConfigValidators.
func (s *K3sServerResource) ConfigValidators(ctx context.Context) []resource.ConfigValidator {
	return []resource.ConfigValidator{
		&k3sServerAuthValdiator{},
	}
}

func (s *K3sServerResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	data := ServerClientModel{
		BinDir: types.StringValue("/usr/local/bin"),
		Port:   types.Int32Value(22),
	}

	for field := range strings.SplitSeq(req.ID, ",") {
		kv := strings.Split(field, "=")
		if len(kv) != 2 {
			resp.Diagnostics.AddError("failed importing", "Importing k3s_server requires comma separated field=value")
		}
		if kv[0] == "host" {
			data.Host = types.StringValue(kv[1])
		}
		if kv[0] == "user" {
			data.User = types.StringValue(kv[1])
		}
		if kv[0] == "private_key" {
			data.PrivateKey = types.StringValue(kv[1])
		}
		if kv[0] == "port" {
			val, err := strconv.Atoi(kv[1])
			if err != nil {
				resp.Diagnostics.AddError("failed importing", "Could not parse port")
				return
			}
			data.Port = types.Int32Value(int32(val))
		}
		if kv[0] == "password" {
			data.Password = types.StringValue(kv[1])
		}
		if kv[0] == "binDir" {
			data.BinDir = types.StringValue(kv[1])
		}
		if kv[0] == "cluster_init" {
			haConfig := HaConfig{}
			if !data.HaConfig.IsNull() {
				data.HaConfig.As(ctx, &haConfig, basetypes.ObjectAsOptions{})
			}
			haConfig.ClusterInit = types.BoolValue(kv[1] == "true")
			data.HaConfig, _ = types.ObjectValueFrom(ctx, haConfig.AttributeTypes(), haConfig)
		}
	}

	sshClient, err := data.sshClient(ctx)
	if err != nil {
		resp.Diagnostics.AddError("failed importing: Creating ssh config", err.Error())
		return
	}

	tflog.Info(ctx, "Resyncing k3s_server")
	server := k3s.NewK3sServerComponent(ctx, nil, nil, nil, data.BinDir.ValueString())
	if err := server.Resync(sshClient); err != nil {
		resp.Diagnostics.AddError("failed importing: resyncing k3s_server", err.Error())
		return
	}

	tflog.Info(ctx, "Checking k3s systemd status")
	active, err := server.Status(sshClient)
	if err != nil {
		resp.Diagnostics.AddError("failed importing: Error retrieving server status", err.Error())
		return
	}

	data.Active = types.BoolValue(active)
	data.KubeConfig = types.StringValue(server.KubeConfig())
	data.Token = types.StringValue(server.Token())

	if !data.HaConfig.IsNull() {
		var haConfig HaConfig
		data.HaConfig.As(ctx, &haConfig, basetypes.ObjectAsOptions{})
		if !haConfig.ClusterInit.ValueBool() {
			if server, ok := server.Config()["server"].(string); ok {
				haConfig.Server = types.StringValue(server)
			}
			haConfig.Token = types.StringValue(server.Token())
		}
		data.HaConfig, _ = types.ObjectValueFrom(ctx, haConfig.AttributeTypes(), haConfig)
	}

	if serverConfig := server.Config(); serverConfig != nil {
		config, err := yaml.Marshal(serverConfig)
		if err != nil {
			resp.Diagnostics.AddError("failed importing: Error server config", err.Error())
			return
		}
		data.K3sConfig = types.StringValue(string(config))
	}

	if registry := server.Registry(); registry != nil {
		contents, err := yaml.Marshal(registry)
		if err != nil {
			resp.Diagnostics.AddError("failed importing: Error server registry", err.Error())
			return
		}
		if string(contents) != "" {
			data.K3sRegistry = types.StringValue(string(contents))
		}
	}

	tflog.Info(ctx, "Imported a k3s server resource")
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), types.StringValue(fmt.Sprintf("server,%s", data.Host)))...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

type k3sServerAuthValdiator struct{}

var _ resource.ConfigValidator = &k3sServerAuthValdiator{}

// Description implements resource.ConfigValidator.
func (k *k3sServerAuthValdiator) Description(context.Context) string {
	return "Validates the authentication for the server"
}

// MarkdownDescription implements resource.ConfigValidator.
func (k *k3sServerAuthValdiator) MarkdownDescription(context.Context) string {
	return "Allows either Password or Private Key, but not both"
}

// ValidateResource implements resource.ConfigValidator.
func (k *k3sServerAuthValdiator) ValidateResource(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var data ServerClientModel

	// Read Terraform state data into the model
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)

	if data.PrivateKey.IsNull() && data.Password.IsNull() {
		resp.Diagnostics.AddError("No auth", "Neither password nor private key was passed")
		return
	}

	if !data.PrivateKey.IsNull() && !data.Password.IsNull() {
		resp.Diagnostics.AddError("Conflicting auth", "Both password and private key were passed, only pass one")
		return
	}

	if !data.HaConfig.IsNull() && !data.HaConfig.IsUnknown() {
		var haConfig HaConfig
		data.HaConfig.As(ctx, &haConfig, basetypes.ObjectAsOptions{})
		if !haConfig.ClusterInit.ValueBool() && (haConfig.Token.IsNull() || haConfig.Server.IsNull()) {
			resp.Diagnostics.AddError("Highly available", "When not in cluster-init, token and server must be passed")
			return
		}
		if haConfig.ClusterInit.ValueBool() && (!haConfig.Token.IsNull() || !haConfig.Server.IsNull()) {
			resp.Diagnostics.AddError("Highly available", "When in cluster-init, token and server must not be passed")
			return
		}
	}
}
