package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"gopkg.in/yaml.v2"
	"striveworks.us/terraform-provider-k3s/internal/k3s"
	"striveworks.us/terraform-provider-k3s/internal/ssh_client"
)

var _ resource.ResourceWithConfigure = &K3sAgentResource{}
var _ resource.ResourceWithConfigValidators = &K3sAgentResource{}

type K3sAgentResource struct {
	version *string
}

// AgentClientModel describes the resource data model.
type AgentClientModel struct {
	Auth types.Object `tfsdk:"auth"`
	// Connection
	Server types.String `tfsdk:"server"`
	BinDir types.String `tfsdk:"bin_dir"`
	// Configs
	KubeConfig types.String `tfsdk:"kubeconfig"`
	K3sConfig  types.String `tfsdk:"config"`
	Token      types.String `tfsdk:"token"`
	// Outputs
	Id     types.String `tfsdk:"id"`
	Active types.Bool   `tfsdk:"active"`
}

func (s *AgentClientModel) sshClient(ctx context.Context) (ssh_client.SSHClient, error) {
	auth := NewNodeAuth(ctx, s.Auth)

	port := 22
	if int(auth.Port.ValueInt32()) != 0 {
		port = int(auth.Port.ValueInt32())
	}

	return ssh_client.NewSSHClient(
		ctx,
		auth.Host.ValueString(),
		port,
		auth.User.ValueString(),
		auth.PrivateKey.ValueString(),
		auth.Password.ValueString(),
	)
}

func (s *AgentClientModel) buildAgent(ctx context.Context, version *string) (k3s.K3sAgent, error) {
	config := make(map[any]any)
	if err := yaml.Unmarshal([]byte(s.K3sConfig.ValueString()), &config); err != nil {
		return nil, err
	}

	return k3s.NewK3sAgentComponent(
		ctx,
		config,
		version,
		s.Token.ValueString(),
		s.Server.ValueString(),
		s.BinDir.ValueString(),
	), nil
}

// Schema implements resource.Resource.
func (k *K3sAgentResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Creates a k3s agent resource. Only one of `password` or `private_key` can be passed. Requires a token and server address to a k3s_server resource",

		Attributes: map[string]schema.Attribute{

			// Inputs
			"bin_dir": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Value of a path used to put the k3s binary",
				Default:             stringdefault.StaticString("/usr/local/bin"),
				Computed:            true,
			},
			// Auth
			"auth": NodeAuth{}.Schema(),
			// Config
			"config": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "K3s server config",
			},
			"token": schema.StringAttribute{
				Required:            true,
				Sensitive:           true,
				MarkdownDescription: "Server token used for joining nodes to the cluster",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"server": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Hostname for k3s api server",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"kubeconfig": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "KubeConfig for the cluster, needed so agent node can clean itself up",
			},
			// Outputs
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Id of the k3s server resource",
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

// Configure implements resource.ResourceWithConfigure.
func (k *K3sAgentResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	provider, ok := req.ProviderData.(*K3sProvider)
	if !ok {
		resp.Diagnostics.AddError("Could not convert provider data into version", "")
		return
	}
	if provider.Version != "" {
		k.version = &provider.Version
	}
}

// Create implements resource.Resource.
func (k *K3sAgentResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data AgentClientModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	if data.Token.ValueString() == "" || data.Server.ValueString() == "" {
		resp.Diagnostics.AddError("empty args", "Token or server cannot be empty strings")
		return
	}

	sshClient, err := data.sshClient(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Creating ssh config", err.Error())
		return
	}

	agent, err := data.buildAgent(ctx, k.version)
	if err != nil {
		resp.Diagnostics.AddError("building k3s agent", err.Error())
		return
	}

	if err := agent.RunPreReqs(sshClient); err != nil {
		resp.Diagnostics.AddError("running k3s agent prereqs", err.Error())
		return
	}

	if err := agent.RunInstall(sshClient); err != nil {
		resp.Diagnostics.AddError("running k3s agent install", err.Error())
		return
	}

	active, err := agent.Status(sshClient)
	if err != nil {
		resp.Diagnostics.AddError("retrieving agent status", err.Error())
		return
	}

	if !active {
		status, err := agent.StatusLog(sshClient)
		if err != nil {
			resp.Diagnostics.AddError("retrieving systemctl status", err.Error())
			return
		}
		resp.Diagnostics.AddError("running k3s-agent systemctl status", status)

		logs, err := agent.Journal(sshClient)
		if err != nil {
			resp.Diagnostics.AddError("retrieving journalctl status", err.Error())
			return
		}
		tflog.Trace(ctx, logs)
	}
	auth := NewNodeAuth(ctx, data.Auth)

	data.Active = types.BoolValue(active)
	data.Id = types.StringValue(fmt.Sprintf("agent,%s", auth.Host.ValueString()))

	tflog.Info(ctx, "created a k3s agent resource")

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)

}

// Delete implements resource.Resource.
func (k *K3sAgentResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data AgentClientModel

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

	agent := k3s.NewK3sAgentComponent(ctx, nil, nil, "", "", data.BinDir.ValueString())
	if err := agent.RunUninstall(sshClient, data.KubeConfig.ValueString()); err != nil {
		resp.Diagnostics.AddError("Creating uninstall k3s-agent", err.Error())
		return
	}
}

// Metadata implements resource.Resource.
func (k *K3sAgentResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_agent"
}

// Read implements resource.Resource.
func (k *K3sAgentResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data AgentClientModel

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
	agent, err := data.buildAgent(ctx, k.version)
	if err != nil {
		resp.Diagnostics.AddError("building k3s agent", err.Error())
		return
	}

	if err := agent.Resync(sshClient); err != nil {
		resp.Diagnostics.AddError("Resyncing k3s_agent", err.Error())
		return
	}

	active, err := agent.Status(sshClient)
	if err != nil {
		resp.Diagnostics.AddError("Error retrieving agent status", err.Error())
		return
	}
	data.Active = types.BoolValue(active)
	data.Server = types.StringValue(agent.Server())
	data.Token = types.StringValue(agent.Token())

	resp.Diagnostics.Append(req.State.Set(ctx, &data)...)
}

// Update implements resource.Resource.
func (k *K3sAgentResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data AgentClientModel
	var state AgentClientModel

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

	agent, err := data.buildAgent(ctx, k.version)
	if err != nil {
		resp.Diagnostics.AddError("building k3s agent", err.Error())
		return
	}
	if err := agent.Update(sshClient); err != nil {
		resp.Diagnostics.AddError("Creating uninstall k3s", err.Error())
		return
	}
}

// ConfigValidators implements resource.ResourceWithConfigValidators.
func (k *K3sAgentResource) ConfigValidators(ctx context.Context) []resource.ConfigValidator {
	return []resource.ConfigValidator{
		&k3sAgentAuthValidator{},
	}
}

func NewK3sAgentResource() resource.Resource {
	return &K3sAgentResource{}
}

// Validation

type k3sAgentAuthValidator struct{}

var _ resource.ConfigValidator = &k3sAgentAuthValidator{}

// Description implements resource.ConfigValidator.
func (k *k3sAgentAuthValidator) Description(context.Context) string {
	return "Validates the authentication for the agent"
}

// MarkdownDescription implements resource.ConfigValidator.
func (k *k3sAgentAuthValidator) MarkdownDescription(context.Context) string {
	return "Allows either Password or Private Key, but not both"
}

// ValidateResource implements resource.ConfigValidator.
func (k *k3sAgentAuthValidator) ValidateResource(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var data AgentClientModel

	// Read Terraform state data into the model
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)

	if err := NewNodeAuth(ctx, data.Auth).Validate(); err != nil {
		resp.Diagnostics.AddError("Auth error", err.Error())
		return
	}
}
