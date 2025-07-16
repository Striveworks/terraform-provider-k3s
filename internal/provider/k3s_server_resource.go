package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"gopkg.in/yaml.v2"
	"striveworks.us/terraform-provider-k3s/internal/k3s"
)

// Ensure structs are properly implements interfaces

var _ resource.ResourceWithConfigValidators = &K3sServerResource{}
var _ resource.ResourceWithConfigure = &K3sServerResource{}

type K3sServerResource struct {
	version *string
}

func NewK3sServerResource() resource.Resource {
	return &K3sServerResource{}
}

// ServerClientModel describes the resource data model.
type ServerClientModel struct {
	Auth types.Object `tfsdk:"auth"`
	// Configs
	BinDir      types.String `tfsdk:"bin_dir"`
	K3sConfig   types.String `tfsdk:"config"`
	K3sRegistry types.String `tfsdk:"registry"`
	// Highly Available config
	HaConfig types.Object `tfsdk:"highly_available"`
	// OIDC Support
	OidcConfig types.Object `tfsdk:"oidc"`
	// Outputs
	Id          types.String `tfsdk:"id"`
	Server      types.String `tfsdk:"server"`
	KubeConfig  types.String `tfsdk:"kubeconfig"`
	Token       types.String `tfsdk:"token"`
	Active      types.Bool   `tfsdk:"active"`
	ClusterAuth types.Object `tfsdk:"cluster_auth"`
}

type ServerBuilder struct {
	model   ServerClientModel
	Ha      *HaConfig
	Oidc    *OidcConfig
	version *string
}

func NewServerBuilder(model ServerClientModel, version *string) ServerBuilder {
	return ServerBuilder{model: model, version: version}
}

func (s *ServerBuilder) Build(ctx context.Context, d *diag.Diagnostics) k3s.K3sServer {

	config, err := ParseYamlString(s.model.K3sConfig)
	if err != nil {
		d.AddError("parsing config", err.Error())
		return nil
	}
	tflog.Debug(ctx, "K3s config parsed")

	registry, err := ParseYamlString(s.model.K3sRegistry)
	if err != nil {
		d.AddError("parsing registry", err.Error())
		return nil
	}
	config["embedded-registry"] = registry != nil
	tflog.Debug(ctx, "K3s registry parsed")

	// Oidc Support?

	if !s.model.OidcConfig.IsNull() {
		tflog.Debug(ctx, "See OIDC support requested, adding to config")
		d.Append(s.model.OidcConfig.As(ctx, &s.Oidc, basetypes.ObjectAsOptions{})...)
		if d.HasError() {
			return nil
		}

		config["kube-apiserver-arg"] = []string{
			fmt.Sprintf("api-audiences=%s", s.Oidc.Audience.ValueString()),
			"service-account-key-file=/etc/rancher/k3s/tls/sa-signer-pkcs8.pub",
			"service-account-key-file=/var/lib/rancher/k3s/server/tls/service.key",
			"service-account-signing-key-file=/etc/rancher/k3s/tls/sa-signer.key",
			fmt.Sprintf("service-account-issuer=%s", s.Oidc.Issuer.ValueString()),
			"service-account-issuer=k3s",
		}

	}

	var server k3s.K3sServer
	if !s.model.HaConfig.IsNull() {
		d.Append(s.model.HaConfig.As(ctx, &s.Ha, basetypes.ObjectAsOptions{})...)
		if d.HasError() {
			return nil
		}

		if s.Ha.ClusterInit.ValueBool() {
			tflog.Info(ctx, "Running in node HA init mode")
			config["cluster-init"] = true
			server = k3s.NewK3sServerHAComponent(ctx, config, registry, s.version, "", s.model.BinDir.ValueString())
		} else {
			tflog.Info(ctx, "Running in node HA join mode")
			config["server"] = s.Ha.Server.ValueString()
			server = k3s.NewK3sServerHAComponent(ctx, config, registry, s.version, s.Ha.Token.ValueString(), s.model.BinDir.ValueString())
		}
	} else {
		server = k3s.NewK3sServerComponent(ctx, config, registry, s.version, s.model.BinDir.ValueString())
	}

	if s.Oidc != nil {
		server.AddFile("/etc/rancher/k3s/tls/sa-signer-pkcs8.pub", s.Oidc.SigningPKCS8.ValueString())
		server.AddFile("/etc/rancher/k3s/tls/sa-signer.key", s.Oidc.SigningKey.ValueString())
	}

	return server
}

// Schema implements resource.ResourceWithImportState.
func (s *K3sServerResource) Schema(context context.Context, resource resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: ("Creates a k3s server resource. Only one of `password` or `private_key` can be passed.\n" +
			"If ran in highly available mode, it is up to the consumers of this module to correctly implement " +
			"the raft protocol and create an odd number of ha nodes."),
		Attributes: map[string]schema.Attribute{
			"auth": NodeAuth{}.Schema(),
			// Inputs
			"bin_dir": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Value of a path used to put the k3s binary",
				Default:             stringdefault.StaticString("/usr/local/bin"),
				Computed:            true,
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
				Sensitive:           true,
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
			"highly_available": HaConfig{}.Schema(),
			"oidc":             OidcConfig{}.Schema(),
			"cluster_auth":     ClusterAuth{}.Schema(),
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
		resp.Diagnostics.AddError("Provider error", "Could not convert provider data into version")
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
	nodeAuth := NewNodeAuth(ctx, data.Auth)
	sshClient, err := nodeAuth.SshClient(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Creating ssh config", err.Error())
		return
	}
	tflog.Debug(ctx, "SSH client configured")

	builder := NewServerBuilder(data, s.version)
	server := builder.Build(ctx, &resp.Diagnostics)
	if server == nil {
		return
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
	if builder.Ha != nil {
		data.HaConfig = builder.Ha.ToObject(ctx)
	}

	// Set jwks output
	if !data.OidcConfig.IsNull() {
		jwks, err := server.JWKS(sshClient)
		if err != nil {
			resp.Diagnostics.AddError("Getting JWKS key", err.Error())
			return
		}
		builder.Oidc.JWKSKeys = types.StringValue(jwks)
		data.OidcConfig = builder.Oidc.ToObject(ctx)
	}

	authData, err := BuildClusterAuth(server.KubeConfig())
	if err != nil {
		resp.Diagnostics.AddError("malformed kubeconfig", err.Error())
		return
	}
	data.ClusterAuth = authData.ToObject(ctx)
	data.KubeConfig = types.StringValue(server.KubeConfig())
	data.Token = types.StringValue(server.Token())
	data.Id = types.StringValue(fmt.Sprintf("server,%s", nodeAuth.Host.ValueString()))
	data.Server = types.StringValue(fmt.Sprintf("https://%s:6443", nodeAuth.Host.ValueString()))

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

	nodeAuth := NewNodeAuth(ctx, data.Auth)
	sshClient, err := nodeAuth.SshClient(ctx)
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

	nodeAuth := NewNodeAuth(ctx, data.Auth)
	sshClient, err := nodeAuth.SshClient(ctx)
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

	authData, err := BuildClusterAuth(data.KubeConfig.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("malformed kubeconfig", err.Error())
		return
	}

	data.ClusterAuth, _ = types.ObjectValueFrom(ctx, authData.AttributeTypes(), authData)

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
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)

	if resp.Diagnostics.HasError() {
		return
	}

	nodeAuth := NewNodeAuth(ctx, data.Auth)
	sshClient, err := nodeAuth.SshClient(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Creating ssh config", err.Error())
		return
	}

	builder := NewServerBuilder(data, s.version)
	server := builder.Build(ctx, &resp.Diagnostics)
	if server == nil {
		return
	}

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
	if builder.Ha != nil {
		state.HaConfig = builder.Ha.ToObject(ctx)
	}

	if builder.Oidc != nil {
		jwks, err := server.JWKS(sshClient)
		if err != nil {
			resp.Diagnostics.AddError("Getting JWKS key", err.Error())
			return
		}
		builder.Oidc.JWKSKeys = types.StringValue(jwks)
		state.OidcConfig = builder.Oidc.ToObject(ctx)
	}

	authData, err := BuildClusterAuth(data.KubeConfig.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("malformed kubeconfig", err.Error())
		return
	}

	data.ClusterAuth, _ = types.ObjectValueFrom(ctx, authData.AttributeTypes(), authData)

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// ConfigValidators implements resource.ResourceWithConfigValidators.
func (s *K3sServerResource) ConfigValidators(ctx context.Context) []resource.ConfigValidator {
	return []resource.ConfigValidator{
		&k3sServerAuthValdiator{},
	}
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

	if err := NewNodeAuth(ctx, data.Auth).Validate(); err != nil {
		resp.Diagnostics.AddError("No auth", err.Error())
		return
	}

	if !data.HaConfig.IsNull() && !data.HaConfig.IsUnknown() {
		if err := NewHaConfig(ctx, data.HaConfig).Validate(); err != nil {
			resp.Diagnostics.AddError("Highly available", err.Error())
			return
		}
	}
}
