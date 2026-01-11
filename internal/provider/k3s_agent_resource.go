package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"striveworks.us/terraform-provider-k3s/internal/handlers"
	"striveworks.us/terraform-provider-k3s/internal/k3s"
)

var _ resource.ResourceWithConfigure = &K3sAgentResource{}
var _ resource.ResourceWithConfigValidators = &K3sAgentResource{}

type K3sAgentResource struct {
	version *string
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
			"auth": handlers.NodeAuth{}.Schema(),
			// Config
			"env": schema.MapAttribute{
				Optional:            true,
				Sensitive:           true,
				MarkdownDescription: "Extra environment variables to pass to the process",
				ElementType:         types.StringType,
			},
			"config": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "K3s server config",
			},
			"registry": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "K3s agent registry",
			},
			"allow_delete_err": schema.BoolAttribute{
				Optional:            true,
				MarkdownDescription: "If this is true, deleting the node using kubectl first will be allowed to error not stopping the k3s uninstall process",
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
	var data handlers.AgentClientModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	data.SetVersion(k.version)

	auth := handlers.NewNodeAuth(ctx, data.Auth)
	agent, diags := data.ToAgent(ctx)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	if err := data.Create(ctx, &auth, agent); err != nil {
		resp.Diagnostics.AddError("installing k3s agent", err.Error())
		return
	}

	tflog.Info(ctx, "created a k3s agent resource")
	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)

}

// Delete implements resource.Resource.
func (k *K3sAgentResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data handlers.AgentClientModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	auth := handlers.NewNodeAuth(ctx, data.Auth)
	agent := k3s.NewK3sAgentUninstall(ctx, data.BinDir.ValueString())

	if err := data.Delete(ctx, &auth, agent); err != nil {
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
	var data handlers.AgentClientModel

	// Read Terraform state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	data.SetVersion(k.version)
	if resp.Diagnostics.HasError() {
		return
	}

	auth := handlers.NewNodeAuth(ctx, data.Auth)
	agent, diags := data.ToAgent(ctx)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	if err := data.Read(ctx, &auth, agent); err != nil {
		resp.Diagnostics.AddError("Resyncing k3s_agent", err.Error())
		return
	}

	resp.Diagnostics.Append(req.State.Set(ctx, &data)...)
}

// Update implements resource.Resource.
func (k *K3sAgentResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data handlers.AgentClientModel
	var state handlers.AgentClientModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	auth := handlers.NewNodeAuth(ctx, data.Auth)
	agent, diags := data.ToAgent(ctx)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}
	if err := state.Update(ctx, data, &auth, agent); err != nil {
		resp.Diagnostics.AddError("updating k3s agent", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
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
	var data handlers.AgentClientModel

	// Read Terraform state data into the model
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)

	if err := handlers.NewNodeAuth(ctx, data.Auth).Validate(); err != nil {
		resp.Diagnostics.AddError("Auth error", err.Error())
		return
	}

	// Null is bad, Unknown is fine (because planning server+agent together)
	if data.Token.IsNull() || data.Server.IsNull() {
		resp.Diagnostics.AddError("empty args", "Token or server cannot be empty strings")
		return
	}

}
