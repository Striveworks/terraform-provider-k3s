// Copyright (c) HashiCorp, Inc.

package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/go-uuid"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

var _ resource.Resource = &K3sServerResource{}
var _ resource.ResourceWithImportState = &K3sServerResource{}

type K3sServerResource struct{}

func NewK3sServerResource() resource.Resource {
	return &K3sServerResource{}
}

// ServerClientModel describes the resource data model.
type ServerClientModel struct {
	PrivateKey types.String `tfsdk:"private_key"`
	User       types.String `tfsdk:"user"`
	Host       types.String `tfsdk:"host"`
	Id         types.String `tfsdk:"id"`
	KubeConfig types.String `tfsdk:"kubeconfig"`
	Token      types.String `tfsdk:"token"`
	Active     types.Bool   `tfsdk:"active"`
}

func (s ServerClientModel) SshUser() string {
	return s.User.ValueString()
}
func (s ServerClientModel) Pem() string {
	return s.PrivateKey.ValueString()
}
func (s ServerClientModel) Address() string {
	return fmt.Sprintf("%s:22", s.Host.ValueString())
}

// Create implements resource.ResourceWithImportState.
func (s *K3sServerResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data ServerClientModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	logger := func(out string) {
		tflog.Info(ctx, out)
	}

	k3sClient, err := NewK3sClient(data)
	if err != nil {
		resp.Diagnostics.Append(fromError("Creating ssh config", err))
		return
	}

	tflog.Debug(ctx, "Running K3s Server Preq install")
	err = k3sClient.Prereqs(logger)
	if err != nil {
		resp.Diagnostics.Append(fromError("Creating ssh config", err))
		return
	}

	tflog.Debug(ctx, "Running K3s Server install")
	k3sClient.InstallServer(logger)

	kubeconfig, err := k3sClient.KubeConfig()
	if err != nil {
		resp.Diagnostics.Append(fromError("retrieving kubeconfig", err))
		return
	}

	serverToken, err := k3sClient.ServerToken()
	if err != nil {
		resp.Diagnostics.Append(fromError("retrieving servertoken", err))
		return
	}

	active, err := k3sClient.IsActive()
	if err != nil {
		resp.Diagnostics.Append(fromError("Error retrieving server status", err))
		return
	}

	data.Active = types.BoolValue(active)
	data.KubeConfig = types.StringValue(kubeconfig)
	data.Token = types.StringValue(serverToken)
	id, _ := uuid.GenerateUUID()
	data.Id = types.StringValue(id)

	// Write logs using the tflog package
	// Documentation: https://terraform.io/plugin/log
	tflog.Info(ctx, "created a k3s server resource")

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

	k3sClient, err := NewK3sClient(data)
	if err != nil {
		resp.Diagnostics.Append(fromError("Creating ssh config", err))
		return
	}
	err = k3sClient.DeleteServer(func(out string) {
		tflog.Info(ctx, out)
	})
	if err != nil {
		resp.Diagnostics.Append(fromError("deleting k3s server", err))
		return
	}
}

// ImportState implements resource.ResourceWithImportState.
func (s *K3sServerResource) ImportState(context.Context, resource.ImportStateRequest, *resource.ImportStateResponse) {
	panic("unimplemented")
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

	k3sClient, err := NewK3sClient(data)
	if err != nil {
		resp.Diagnostics.Append(fromError("Creating ssh config", err))
		return
	}
	active, err := k3sClient.IsActive()
	if err != nil {
		resp.Diagnostics.Append(fromError("Error retrieving server status", err))
		return
	}
	data.Active = types.BoolValue(active && false)

	resp.Diagnostics.Append(req.State.Set(ctx, &data)...)
}

// Schema implements resource.ResourceWithImportState.
func (s *K3sServerResource) Schema(context context.Context, resource resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: `Creates a K3s Server
Example:
` + "```terraform" + `
resource "k3s_server" "main" {
  host        = "192.168.10.1"
  user        = "ubuntu"
  private_key = var.private_key_openssh
}
` + "```",

		Attributes: map[string]schema.Attribute{
			// Inputs
			"private_key": schema.StringAttribute{
				Sensitive:           true,
				Required:            true,
				MarkdownDescription: "Value of a privatekey used to auth",
			},
			"host": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Hostname of the target server",
			},
			"user": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Username of the target server",
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

// Update implements resource.ResourceWithImportState.
func (s *K3sServerResource) Update(context.Context, resource.UpdateRequest, *resource.UpdateResponse) {
	panic("unimplemented")
}
