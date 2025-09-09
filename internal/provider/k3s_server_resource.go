package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"striveworks.us/terraform-provider-k3s/internal/handlers"
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

// Schema implements resource.ResourceWithImportState.
func (s *K3sServerResource) Schema(context context.Context, resource resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: ("Creates a k3s server resource. Only one of `password` or `private_key` can be passed.\n" +
			"If ran in highly available mode, it is up to the consumers of this module to correctly implement " +
			"the raft protocol and create an odd number of ha nodes. Due to how HA works, we do not offer a method to " +
			"gracefully delete a controller node from the cluster before running `k3s-uninstall.sh` during deletion of this resource."),
		Attributes: map[string]schema.Attribute{
			"auth": handlers.NodeAuth{}.Schema(),
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
			"highly_available": handlers.HaConfig{}.Schema(),
			"oidc":             handlers.OidcConfig{}.Schema(),
			"cluster_auth":     handlers.ClusterAuth{}.Schema(),
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
	var data handlers.ServerClientModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	data.SetVersion(s.version)

	auth := handlers.NewNodeAuth(ctx, data.Auth)
	server, err := data.ToServer(ctx)
	if err != nil {
		resp.Diagnostics.AddError("creating k3s server", err.Error())
		return
	}

	if err := data.Create(ctx, &auth, server); err != nil {
		resp.Diagnostics.AddError("creating k3s server", err.Error())
		return
	}

	tflog.Info(ctx, "Created a k3s server resource")
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// Delete implements resource.ResourceWithImportState.
func (s *K3sServerResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data handlers.ServerClientModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	auth := handlers.NewNodeAuth(ctx, data.Auth)
	server, err := data.ToServer(ctx)
	if err != nil {
		resp.Diagnostics.AddError("creating k3s server", err.Error())
		return
	}

	if err := data.Delete(ctx, &auth, server); err != nil {
		resp.Diagnostics.AddError("deleting k3s server", err.Error())
		return
	}
}

// Metadata implements resource.ResourceWithImportState.
func (s *K3sServerResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_server"
}

// Read implements resource.ResourceWithImportState.
func (s *K3sServerResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data handlers.ServerClientModel

	// Read Terraform state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	data.SetVersion(s.version)

	auth := handlers.NewNodeAuth(ctx, data.Auth)
	server, err := data.ToServer(ctx)
	if err != nil {
		resp.Diagnostics.AddError("creating k3s server", err.Error())
		return
	}

	if err := data.Read(ctx, &auth, server); err != nil {
		resp.Diagnostics.AddError("reading k3s server", err.Error())
		return
	}

	resp.Diagnostics.Append(req.State.Set(ctx, &data)...)
}

// Update implements resource.ResourceWithImportState.
func (s *K3sServerResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data handlers.ServerClientModel
	var state handlers.ServerClientModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)

	data.SetVersion(s.version)
	state.SetVersion(s.version)

	if resp.Diagnostics.HasError() {
		return
	}

	auth := handlers.NewNodeAuth(ctx, data.Auth)
	server, err := data.ToServer(ctx)
	if err != nil {
		resp.Diagnostics.AddError("creating k3s server", err.Error())
		return
	}

	if err := state.Update(ctx, data, &auth, server); err != nil {
		resp.Diagnostics.AddError("updating k3s server", err.Error())
		return
	}

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
	var data handlers.ServerClientModel

	// Read Terraform state data into the model
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)

	if err := handlers.NewNodeAuth(ctx, data.Auth).Validate(); err != nil {
		resp.Diagnostics.AddError("No auth", err.Error())
		return
	}

	if !data.HaConfig.IsNull() && !data.HaConfig.IsUnknown() {
		if err := handlers.NewHaConfig(ctx, data.HaConfig).Validate(); err != nil {
			resp.Diagnostics.AddError("Highly available", err.Error())
			return
		}
	}
}
