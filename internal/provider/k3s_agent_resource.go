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

	"striveworks.us/terraform-provider-k3s/internal/k3s"
	"striveworks.us/terraform-provider-k3s/internal/ssh_client"
)

// Ensure structs are properly implements interfaces

var (
	_ resource.ResourceWithConfigValidators = &K3sAgentResource{}
	_ resource.Resource                     = &K3sAgentResource{}
	_ resource.ConfigValidator              = &K3sAgentResource{}
	_ resource.ResourceWithImportState      = &K3sAgentResource{}
)

type K3sAgentResource struct{}

type AgentClientModel struct {
	// Inputs
	Version     types.String `tfsdk:"version"`
	Auth        types.Object `tfsdk:"auth"`
	BinDir      types.String `tfsdk:"bin_dir"`
	K3sConfig   types.String `tfsdk:"config"`
	K3sRegistry types.String `tfsdk:"registry"`
	Env         types.Map    `tfsdk:"env"`
	Server      types.String `tfsdk:"server"`
	Token       types.String `tfsdk:"token"`

	// Outputs
	Id     types.String `tfsdk:"id"`
	Active types.Bool   `tfsdk:"active"`
}

func NewK3sAgentResource() resource.Resource {
	return &K3sAgentResource{}
}

// ImportState implements [resource.ResourceWithImportState].
func (k *K3sAgentResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
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

	agent := k3s.Agent{}
	exists, active, err := agent.Refresh(ctx, sshClient)
	if err != nil {
		resp.Diagnostics.AddError("importing k3s agent", err.Error())
		return
	}
	if !exists {
		resp.Diagnostics.AddError("importing k3s agent", fmt.Sprintf("no k3s agent found on %s", sshClient.Host()))
		return
	}

	data := AgentClientModel{
		Version:     types.StringValue(agent.Version),
		Auth:        sshConfig.ToObject(ctx),
		BinDir:      types.StringValue(binDir),
		K3sConfig:   types.StringNull(),
		K3sRegistry: types.StringNull(),
		Env:         types.MapNull(types.StringType),
		Id:          types.StringValue(sshClient.Host()),
		Server:      types.StringValue(agent.Server),
		Token:       types.StringValue(agent.Token),
		Active:      types.BoolValue(active),
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// Description implements [resource.ConfigValidator].
func (k *K3sAgentResource) Description(context.Context) string {
	return "Validates the authentication and required connection information for the agent"
}

// MarkdownDescription implements [resource.ConfigValidator].
func (k *K3sAgentResource) MarkdownDescription(context.Context) string {
	return "Requires at least one SSH credential source, a server URL, and a token"
}

// ValidateResource implements [resource.ConfigValidator].
func (k *K3sAgentResource) ValidateResource(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var data AgentClientModel

	tflog.Trace(ctx, "Deserializing AgentClientModel")
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	validateAgentAndReturn(ctx, data, &resp.Diagnostics)
}

// ConfigValidators implements [resource.ResourceWithConfigValidators].
func (k *K3sAgentResource) ConfigValidators(context.Context) []resource.ConfigValidator {
	return []resource.ConfigValidator{&K3sAgentResource{}}
}

// Create implements [resource.ResourceWithConfigValidators].
func (k *K3sAgentResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data AgentClientModel

	tflog.Trace(ctx, "Deserializing AgentClientModel")
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	sshConfig := validateAgentAndReturn(ctx, data, &resp.Diagnostics)
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

	if data.Token.ValueString() != "" {
		tflog.MaskMessageStrings(ctx, data.Token.ValueString())
	}

	agent := k3s.Agent{
		Config:   data.K3sConfig.ValueString(),
		Registry: data.K3sRegistry.ValueString(),
		Token:    data.Token.ValueString(),
		Version:  data.Version.ValueString(),
		BinDir:   data.BinDir.ValueString(),
		Env:      env,
		Server:   data.Server.ValueString(),
	}

	if err := agent.Validate(ctx); err != nil {
		resp.Diagnostics.AddError("validating k3s agent", err.Error())
		return
	}

	if err := agent.PreInstall(ctx, sshClient); err != nil {
		resp.Diagnostics.AddError("running k3s agent preinstall", err.Error())
		return
	}
	if err := agent.Install(ctx, sshClient); err != nil {
		resp.Diagnostics.AddError("running k3s agent install", err.Error())
		return
	}

	exists, active, err := agent.Refresh(ctx, sshClient)
	if err != nil {
		resp.Diagnostics.AddError("refreshing k3s agent", err.Error())
		return
	}
	if !exists {
		resp.Diagnostics.AddError("refreshing k3s agent", fmt.Sprintf("no k3s agent found on %s after install", sshClient.Host()))
		return
	}

	populateAgentState(&data, agent, sshClient, active)

	tflog.Info(ctx, "Created a k3s agent resource")
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// Delete implements [resource.ResourceWithConfigValidators].
func (k *K3sAgentResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data AgentClientModel

	tflog.Trace(ctx, "Deserializing AgentClientModel")
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

	agent := k3s.Agent{
		BinDir: data.BinDir.ValueString(),
	}

	if err := agent.Uninstall(ctx, sshClient); err != nil {
		resp.Diagnostics.AddError("deleting k3s agent", err.Error())
		return
	}

	resp.State.RemoveResource(ctx)
}

// Metadata implements [resource.ResourceWithConfigValidators].
func (k *K3sAgentResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_agent"
}

// Read implements [resource.ResourceWithConfigValidators].
func (k *K3sAgentResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data AgentClientModel

	tflog.Trace(ctx, "Deserializing AgentClientModel")
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

	agent := k3s.Agent{
		BinDir: data.BinDir.ValueString(),
	}
	exists, active, err := agent.Refresh(ctx, sshClient)
	if err != nil {
		resp.Diagnostics.AddError("reading k3s agent", err.Error())
		return
	}
	if !exists {
		resp.State.RemoveResource(ctx)
		return
	}

	populateAgentState(&data, agent, sshClient, active)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// Schema implements [resource.ResourceWithConfigValidators].
func (k *K3sAgentResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Creates a k3s agent resource. Requires SSH authentication, a token, and a server address from a k3s_server resource.",
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
				MarkdownDescription: "K3s agent config",
			},
			"env": schema.MapAttribute{
				Optional:            true,
				Sensitive:           true,
				MarkdownDescription: "Extra environment variables to pass to the process",
				ElementType:         types.StringType,
			},
			"registry": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "K3s agent registry",
			},
			"token": schema.StringAttribute{
				Required:            true,
				Sensitive:           true,
				MarkdownDescription: "Server token used for joining nodes to the cluster.",
			},
			"server": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Server url used for joining nodes to the cluster.",
			},
			// Outputs
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Id of the k3s agent resource",
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
		},
	}
}

// Update implements [resource.ResourceWithConfigValidators].
func (k *K3sAgentResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data AgentClientModel

	tflog.Trace(ctx, "Deserializing AgentClientModel")
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	sshConfig := validateAgentAndReturn(ctx, data, &resp.Diagnostics)
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

	if data.Token.ValueString() != "" {
		tflog.MaskMessageStrings(ctx, data.Token.ValueString())
	}

	agent := k3s.Agent{
		Config:   data.K3sConfig.ValueString(),
		Registry: data.K3sRegistry.ValueString(),
		Token:    data.Token.ValueString(),
		Version:  data.Version.ValueString(),
		BinDir:   data.BinDir.ValueString(),
		Env:      env,
		Server:   data.Server.ValueString(),
	}

	if err := agent.Validate(ctx); err != nil {
		resp.Diagnostics.AddError("validating k3s agent", err.Error())
		return
	}

	if err := agent.PreInstall(ctx, sshClient); err != nil {
		resp.Diagnostics.AddError("running k3s agent preinstall", err.Error())
		return
	}
	if err := agent.Update(ctx, sshClient); err != nil {
		resp.Diagnostics.AddError("running k3s agent update", err.Error())
		return
	}

	exists, active, err := agent.Refresh(ctx, sshClient)
	if err != nil {
		resp.Diagnostics.AddError("refreshing k3s agent", err.Error())
		return
	}
	if !exists {
		resp.State.RemoveResource(ctx)
		return
	}

	populateAgentState(&data, agent, sshClient, active)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func validateAgentAndReturn(ctx context.Context, data AgentClientModel, d *diag.Diagnostics) (sshConfig ssh_client.SSHConfig) {
	tflog.Trace(ctx, "Deserializing SSHConfig")
	d.Append(data.Auth.As(ctx, &sshConfig, basetypes.ObjectAsOptions{})...)
	if d.HasError() {
		return
	}

	tflog.Trace(ctx, "Validating SSHConfig")
	if err := sshConfig.Validate(); err != nil {
		d.AddError("validating auth", err.Error())
		return
	}

	if data.Token.IsNull() {
		d.AddError("validating token", "token cannot be null")
		return
	}
	if !data.Token.IsUnknown() && data.Token.ValueString() == "" {
		d.AddError("validating token", "token cannot be an empty string")
		return
	}

	if data.Server.IsNull() {
		d.AddError("validating server", "server cannot be null")
		return
	}
	if !data.Server.IsUnknown() && data.Server.ValueString() == "" {
		d.AddError("validating server", "server cannot be an empty string")
		return
	}

	return
}

func populateAgentState(data *AgentClientModel, agent k3s.Agent, sshClient ssh_client.SSHClient, active bool) {
	data.Version = types.StringValue(agent.Version)
	data.Id = types.StringValue(sshClient.Host())
	data.Server = types.StringValue(agent.Server)
	data.Token = types.StringValue(agent.Token)
	data.Active = types.BoolValue(active)
}
