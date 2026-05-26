package provider

import (
	"context"
	"fmt"
	"net/url"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"striveworks.us/terraform-provider-k3s/internal/k3s"
	"striveworks.us/terraform-provider-k3s/internal/schemas"
	"striveworks.us/terraform-provider-k3s/internal/ssh_client"
)

// Ensure structs are properly implements interfaces

var (
	_ resource.ResourceWithConfigValidators = &K3sServerResource{}
	_ resource.Resource                     = &K3sServerResource{}
	_ resource.ConfigValidator              = &K3sServerResource{}
	_ resource.ResourceWithImportState      = &K3sServerResource{}
)

type K3sServerResource struct{}

type ServerClientModel struct {
	// Inputs
	Version        types.String `tfsdk:"version"`
	Auth           types.Object `tfsdk:"auth"`
	BinDir         types.String `tfsdk:"bin_dir"`
	K3sConfig      types.String `tfsdk:"config"`
	K3sRegistry    types.String `tfsdk:"registry"`
	Env            types.Map    `tfsdk:"env"`
	HaConfig       types.Object `tfsdk:"highly_available"`
	OidcConfig     types.Object `tfsdk:"oidc"`
	BootstrapToken types.String `tfsdk:"bootstrap_token"`
	Orphan         types.Bool   `tfsdk:"orphan"`
	// Outputs
	Id          types.String `tfsdk:"id"`
	Server      types.String `tfsdk:"server"`
	KubeConfig  types.String `tfsdk:"kubeconfig"`
	Token       types.String `tfsdk:"token"`
	Active      types.Bool   `tfsdk:"active"`
	ClusterAuth types.Object `tfsdk:"cluster_auth"`
}

func NewK3sServerResource() resource.Resource {
	return &K3sServerResource{}
}

// ImportState implements [resource.ResourceWithImportState].
func (s *K3sServerResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	sshConfig, binDir, err := parseServerImportID(req.ID)
	if err != nil {
		resp.Diagnostics.AddError("parsing import id", err.Error())
		return
	}

	if err := sshConfig.Validate(); err != nil {
		resp.Diagnostics.AddError("validating auth", err.Error())
		return
	}

	sshClient, err := ssh_client.NewSSHClient(ctx, sshConfig)
	if err != nil {
		resp.Diagnostics.AddError("creating ssh client", err.Error())
		return
	}

	server := k3s.Server{
		BinDir: binDir,
	}
	exists, active, err := server.Refresh(ctx, sshClient)
	if err != nil {
		resp.Diagnostics.AddError("importing k3s server", err.Error())
		return
	}
	if !exists {
		resp.Diagnostics.AddError("importing k3s server", fmt.Sprintf("no k3s service found on %s", sshClient.Host()))
		return
	}

	clusterAuth, err := schemas.BuildClusterAuth(server.KubeConfig)
	if err != nil {
		resp.Diagnostics.AddError("building cluster auth", err.Error())
		return
	}

	data := ServerClientModel{
		Version:        types.StringValue(server.Version),
		Auth:           sshConfig.ToObject(ctx),
		BinDir:         types.StringValue(binDir),
		K3sConfig:      types.StringNull(),
		K3sRegistry:    types.StringNull(),
		Env:            types.MapNull(types.StringType),
		HaConfig:       types.ObjectNull(schemas.HaConfig{}.AttributeTypes()),
		OidcConfig:     types.ObjectNull(schemas.OidcConfig{}.AttributeTypes()),
		BootstrapToken: types.StringNull(),
		Id:             types.StringValue(sshClient.Host()),
		Server:         clusterAuth.Server,
		KubeConfig:     types.StringValue(server.KubeConfig),
		Token:          types.StringValue(server.Token),
		Active:         types.BoolValue(active),
		ClusterAuth:    clusterAuth.ToObject(ctx),
		Orphan:         types.BoolValue(false),
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// Create implements resource.ResourceWithImportState.
func (s *K3sServerResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data ServerClientModel

	// Read Terraform plan data into the model
	tflog.Trace(ctx, "Deserializing ServerClientModel")
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	sshConfig, haConfig, oidcConfig := validateServerAndReturn(ctx, data, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	sshClient, err := ssh_client.NewSSHClient(ctx, sshConfig)
	if err != nil {
		resp.Diagnostics.AddError("creating ssh client", err.Error())
		return
	}

	env := make(map[string]string)
	tflog.Trace(ctx, "Deserializing Env vars")
	if !data.Env.IsNull() && !data.Env.IsUnknown() {
		resp.Diagnostics.Append(data.Env.ElementsAs(ctx, &env, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
	}

	if data.BootstrapToken.ValueString() != "" {
		tflog.MaskMessageStrings(ctx, data.BootstrapToken.ValueString())
	}

	server := k3s.Server{
		Config:   data.K3sConfig.ValueString(),
		Registry: data.K3sRegistry.ValueString(),
		Version:  data.Version.ValueString(),
		BinDir:   data.BinDir.ValueString(),
		Env:      env,
	}
	if !data.BootstrapToken.IsNull() && !data.BootstrapToken.IsUnknown() {
		server.Token = data.BootstrapToken.ValueString()
	}

	if err := server.Validate(ctx); err != nil {
		resp.Diagnostics.AddError("validating k3s server", err.Error())
		return
	}

	if oidcConfig != nil {
		tflog.Debug(ctx, "Running server with oidc config")
		server.WithOidc(*oidcConfig)
	}

	if haConfig != nil {
		tflog.Debug(ctx, "Running server with highly available config")
		server.WithHa(*haConfig)
	}

	if err := server.PreInstall(ctx, sshClient); err != nil {
		resp.Diagnostics.AddError("running k3s server preinstall", err.Error())
		return
	}
	if err := server.Install(ctx, sshClient); err != nil {
		resp.Diagnostics.AddError("running k3s server install", err.Error())
		return
	}

	exists, active, err := server.Refresh(ctx, sshClient)
	if err != nil {
		resp.Diagnostics.AddError("refreshing k3s server", err.Error())
		return
	}
	if !exists {
		resp.Diagnostics.AddError("refreshing k3s server", fmt.Sprintf("no k3s server found on %s after install", sshClient.Host()))
		return
	}

	data.KubeConfig = types.StringValue(server.KubeConfig)
	data.Token = types.StringValue(server.Token)
	data.Version = types.StringValue(server.Version)
	data.Id = types.StringValue(sshClient.Host())
	data.Active = types.BoolValue(active)

	clusterAuth, err := schemas.BuildClusterAuth(server.KubeConfig)
	if err != nil {
		resp.Diagnostics.AddError("building cluster auth", err.Error())
		return
	}
	data.ClusterAuth = clusterAuth.ToObject(ctx)
	data.Server = clusterAuth.Server

	if !setOIDCJWKSKeys(ctx, &data, oidcConfig, server, sshClient, &resp.Diagnostics) {
		return
	}

	tflog.Info(ctx, "Created a k3s server resource")
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// Delete implements resource.ResourceWithImportState.
func (s *K3sServerResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data ServerClientModel

	// Read Terraform state data into the model
	tflog.Trace(ctx, "Deserializing ServerClientModel")
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if data.Orphan.ValueBool() {
		tflog.Info(ctx, "Orphaning k3s server resource without uninstalling")
		resp.State.RemoveResource(ctx)
		return
	}

	var sshConfig ssh_client.SSHConfig
	tflog.Trace(ctx, "Deserializing SSHConfig")
	resp.Diagnostics.Append(data.Auth.As(ctx, &sshConfig, basetypes.ObjectAsOptions{})...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Trace(ctx, "Validating SSHConfig")
	if err := sshConfig.Validate(); err != nil {
		resp.Diagnostics.AddError("validating auth", err.Error())
		return
	}

	sshClient, err := ssh_client.NewSSHClient(ctx, sshConfig)
	if err != nil {
		resp.Diagnostics.AddError("creating ssh client", err.Error())
		return
	}

	server := k3s.Server{
		BinDir: data.BinDir.ValueString(),
	}

	if err := server.Uninstall(ctx, sshClient); err != nil {
		resp.Diagnostics.AddError("deleting k3s server", err.Error())
		return
	}

	resp.State.RemoveResource(ctx)
}

// Metadata implements resource.ResourceWithImportState.
func (s *K3sServerResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_server"
}

// Read implements resource.ResourceWithImportState.
func (s *K3sServerResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data ServerClientModel

	// Read Terraform state data into the model
	tflog.Trace(ctx, "Deserializing ServerClientModel")
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var sshConfig ssh_client.SSHConfig
	tflog.Trace(ctx, "Deserializing SSHConfig")
	resp.Diagnostics.Append(data.Auth.As(ctx, &sshConfig, basetypes.ObjectAsOptions{})...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Trace(ctx, "Validating SSHConfig")
	if err := sshConfig.Validate(); err != nil {
		resp.Diagnostics.AddError("validating auth", err.Error())
		return
	}

	sshClient, err := ssh_client.NewSSHClient(ctx, sshConfig)
	if err != nil {
		resp.Diagnostics.AddError("creating ssh client", err.Error())
		return
	}

	server := k3s.Server{
		BinDir: data.BinDir.ValueString(),
	}
	exists, active, err := server.Refresh(ctx, sshClient)
	if err != nil {
		resp.Diagnostics.AddError("reading k3s server", err.Error())
		return
	}
	if !exists {
		resp.State.RemoveResource(ctx)
		return
	}

	data.KubeConfig = types.StringValue(server.KubeConfig)
	data.Token = types.StringValue(server.Token)
	data.Version = types.StringValue(server.Version)
	data.Id = types.StringValue(sshClient.Host())
	data.Active = types.BoolValue(active)

	clusterAuth, err := schemas.BuildClusterAuth(server.KubeConfig)
	if err != nil {
		resp.Diagnostics.AddError("building cluster auth", err.Error())
		return
	}
	data.ClusterAuth = clusterAuth.ToObject(ctx)
	data.Server = clusterAuth.Server

	var oidcConfig *schemas.OidcConfig
	if !data.OidcConfig.IsNull() && !data.OidcConfig.IsUnknown() {
		resp.Diagnostics.Append(data.OidcConfig.As(ctx, &oidcConfig, basetypes.ObjectAsOptions{})...)
		if resp.Diagnostics.HasError() {
			return
		}
		if !setOIDCJWKSKeys(ctx, &data, oidcConfig, server, sshClient, &resp.Diagnostics) {
			return
		}
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// Update implements resource.ResourceWithImportState.
func (s *K3sServerResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data ServerClientModel

	tflog.Trace(ctx, "Deserializing ServerClientModel")
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	sshConfig, haConfig, oidcConfig := validateServerAndReturn(ctx, data, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	sshClient, err := ssh_client.NewSSHClient(ctx, sshConfig)
	if err != nil {
		resp.Diagnostics.AddError("creating ssh client", err.Error())
		return
	}

	env := make(map[string]string)
	tflog.Trace(ctx, "Deserializing Env vars")
	if !data.Env.IsNull() && !data.Env.IsUnknown() {
		resp.Diagnostics.Append(data.Env.ElementsAs(ctx, &env, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
	}

	if data.BootstrapToken.ValueString() != "" {
		tflog.MaskMessageStrings(ctx, data.BootstrapToken.ValueString())
	}

	server := k3s.Server{
		Config:   data.K3sConfig.ValueString(),
		Registry: data.K3sRegistry.ValueString(),
		Version:  data.Version.ValueString(),
		BinDir:   data.BinDir.ValueString(),
		Env:      env,
	}

	if err := server.Validate(ctx); err != nil {
		resp.Diagnostics.AddError("validating k3s server", err.Error())
		return
	}

	if oidcConfig != nil {
		tflog.Debug(ctx, "Updating server with oidc config")
		server.WithOidc(*oidcConfig)
	}

	if haConfig != nil {
		tflog.Debug(ctx, "Updating server with highly available config")
		server.WithHa(*haConfig)
	}

	if err := server.PreInstall(ctx, sshClient); err != nil {
		resp.Diagnostics.AddError("running k3s server preinstall", err.Error())
		return
	}
	if err := server.Update(ctx, sshClient); err != nil {
		resp.Diagnostics.AddError("running k3s server update", err.Error())
		return
	}

	exists, active, err := server.Refresh(ctx, sshClient)
	if err != nil {
		resp.Diagnostics.AddError("refreshing k3s server", err.Error())
		return
	}
	if !exists {
		resp.State.RemoveResource(ctx)
		return
	}

	data.KubeConfig = types.StringValue(server.KubeConfig)
	data.Token = types.StringValue(server.Token)
	data.Version = types.StringValue(server.Version)
	data.Id = types.StringValue(sshClient.Host())
	data.Active = types.BoolValue(active)

	clusterAuth, err := schemas.BuildClusterAuth(server.KubeConfig)
	if err != nil {
		resp.Diagnostics.AddError("building cluster auth", err.Error())
		return
	}
	data.ClusterAuth = clusterAuth.ToObject(ctx)
	data.Server = clusterAuth.Server

	if !setOIDCJWKSKeys(ctx, &data, oidcConfig, server, sshClient, &resp.Diagnostics) {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)

}

// ConfigValidators implements resource.ResourceWithConfigValidators.
func (s *K3sServerResource) ConfigValidators(ctx context.Context) []resource.ConfigValidator {
	return []resource.ConfigValidator{&K3sServerResource{}}
}

// Description implements resource.ConfigValidator.
func (s *K3sServerResource) Description(context.Context) string {
	return "Validates the authentication for the server"
}

// MarkdownDescription implements resource.ConfigValidator.
func (s *K3sServerResource) MarkdownDescription(context.Context) string {
	return "Requires at least one SSH credential source"
}

// ValidateResource implements resource.ConfigValidator.
func (s *K3sServerResource) ValidateResource(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {

	var data ServerClientModel
	// Read Terraform plan data into the model
	tflog.Trace(ctx, "Deserializing ServerClientModel")
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	validateServerAndReturn(ctx, data, &resp.Diagnostics)
}

func validateServerAndReturn(ctx context.Context, data ServerClientModel, d *diag.Diagnostics) (sshConfig ssh_client.SSHConfig, haConfig *schemas.HaConfig, oidcConfig *schemas.OidcConfig) {

	tflog.Trace(ctx, "Deserializing SSHConfig")
	d.Append(data.Auth.As(ctx, &sshConfig, basetypes.ObjectAsOptions{})...)
	if d.HasError() {
		return
	}

	tflog.Trace(ctx, "Validating SSHConfig")
	if err := (&sshConfig).Validate(); err != nil {
		d.AddError("validating auth", err.Error())
		return
	}

	if !data.BootstrapToken.IsNull() && !data.BootstrapToken.IsUnknown() && data.BootstrapToken.ValueString() == "" {
		d.AddError("validating bootstrap_token", "bootstrap_token cannot be an empty string")
		return
	}

	if !data.HaConfig.IsNull() && !data.HaConfig.IsUnknown() {
		tflog.Trace(ctx, "Deserializing HaConfig")
		d.Append(data.HaConfig.As(ctx, &haConfig, basetypes.ObjectAsOptions{})...)
		if d.HasError() {
			return
		}

		tflog.Trace(ctx, "Validating HaConfig")
		if err := haConfig.Validate(); err != nil {
			d.AddError("validating haConfig", err.Error())
			return
		}
	}

	if !data.OidcConfig.IsNull() && !data.OidcConfig.IsUnknown() {
		tflog.Trace(ctx, "Deserializing oidConfig")
		d.Append(data.OidcConfig.As(ctx, &oidcConfig, basetypes.ObjectAsOptions{})...)
		if d.HasError() {
			return
		}

		tflog.Trace(ctx, "Validating oidConfig")
		if err := oidcConfig.Validate(); err != nil {
			d.AddError("validating oidConfig", err.Error())
			return
		}
	}
	return
}

func setOIDCJWKSKeys(ctx context.Context, data *ServerClientModel, oidcConfig *schemas.OidcConfig, server k3s.Server, sshClient ssh_client.SSHClient, d *diag.Diagnostics) bool {
	if oidcConfig == nil {
		return true
	}

	jwksKeys, err := server.OIDCJWKSKeys(sshClient)
	if err != nil {
		d.AddError("fetching oidc jwks keys", err.Error())
		return false
	}

	oidcConfig.JWKSKeys = types.StringValue(jwksKeys)
	data.OidcConfig = oidcConfig.ToObject(ctx)
	return true
}

// Schema implements resource.ResourceWithImportState.
func (s *K3sServerResource) Schema(context context.Context, resource resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: ("Creates a k3s server resource. At least one of `password`, `private_key`, or `private_key_file` must be provided.\n" +
			"When running in highly available mode, it is up to the consumers of this module to correctly implement " +
			"the raft protocol and create an odd number of ha nodes. Due to how HA works, we do not offer a method to " +
			"gracefully delete a controller node from the cluster before running `k3s-uninstall.sh` during deletion of this resource."),
		Attributes: map[string]schema.Attribute{
			// Version of k3s
			"version": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "The k3s version to use. Versions can be found at https://github.com/k3s-io/k3s/releases. If omitted, the observed running version is stored after install.",
			},
			"auth": ssh_client.SSHConfig{}.Schema(),
			// Inputs
			"bin_dir": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Value of a path used to put the k3s binary",
				Default:             stringdefault.StaticString(k3s.BIN_DIR),
				Computed:            true,
			},
			// Config
			"config": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "K3s server config",
			},
			"env": schema.MapAttribute{
				Optional:            true,
				Sensitive:           true,
				MarkdownDescription: "Extra environment variables to pass to the process",
				ElementType:         types.StringType,
			},
			"bootstrap_token": schema.StringAttribute{
				Optional:            true,
				Sensitive:           true,
				MarkdownDescription: "Short server token used only when bootstrapping a new server. Changing this value requires replacing the server.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"registry": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "K3s server registry",
			},
			"orphan": schema.BoolAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Remove the resource from Terraform state without running the k3s uninstall script during deletion.",
				Default:             booldefault.StaticBool(false),
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
				MarkdownDescription: "Observed server token used for joining nodes to the cluster.",
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
			"highly_available": schemas.HaConfig{}.Schema(),
			"oidc":             schemas.OidcConfig{}.Schema(),
			"cluster_auth":     schemas.ClusterAuth{}.Schema(),
		},
	}
}

func parseServerImportID(rawID string) (ssh_client.SSHConfig, string, error) {
	parsed, err := url.Parse(rawID)
	if err != nil {
		return ssh_client.SSHConfig{}, "", err
	}

	if parsed.Scheme != "ssh" {
		return ssh_client.SSHConfig{}, "", fmt.Errorf("expected import id in the form ssh://user@host[:port]?password=<passworod> or ssh://user@host[:port]?private_key_file=<path>")
	}

	if parsed.User == nil || parsed.User.Username() == "" {
		return ssh_client.SSHConfig{}, "", fmt.Errorf("import id must include the ssh user")
	}

	host := parsed.Hostname()
	if host == "" {
		return ssh_client.SSHConfig{}, "", fmt.Errorf("import id must include the ssh host")
	}

	port := int32(22)
	if parsed.Port() != "" {
		parsedPort, err := strconv.ParseInt(parsed.Port(), 10, 32)
		if err != nil {
			return ssh_client.SSHConfig{}, "", fmt.Errorf("parsing ssh port: %w", err)
		}
		if parsedPort < 1 || parsedPort > 65535 {
			return ssh_client.SSHConfig{}, "", fmt.Errorf("ssh port must be between 1 and 65535")
		}
		port = int32(parsedPort)
	}

	query := parsed.Query()
	password := query.Get("password")
	if password == "" {
		password, _ = parsed.User.Password()
	}

	binDir := query.Get("bin_dir")
	if binDir == "" {
		binDir = k3s.BIN_DIR
	}

	return ssh_client.SSHConfig{
		User:           types.StringValue(parsed.User.Username()),
		Host:           types.StringValue(host),
		Port:           types.Int32Value(port),
		PrivateKey:     optionalImportString(query.Get("private_key")),
		Password:       optionalImportString(password),
		PrivateKeyFile: optionalImportString(query.Get("private_key_file")),
		HostKey:        optionalImportString(query.Get("host_key")),
		HostKeyFile:    optionalImportString(query.Get("host_key_file")),
	}, binDir, nil
}

func optionalImportString(value string) types.String {
	if value == "" {
		return types.StringNull()
	}
	return types.StringValue(value)
}
