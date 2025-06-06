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
	"gopkg.in/yaml.v2"
	"striveworks.us/terraform-provider-k3s/internal/k3s"
	"striveworks.us/terraform-provider-k3s/internal/ssh_client"
)

var _ resource.ResourceWithConfigure = &K3sServerResource{}

type K3sServerResource struct {
	version *string
}

func NewK3sServerResource() resource.Resource {
	return &K3sServerResource{}
}

// ServerClientModel describes the resource data model.
type ServerClientModel struct {
	// Inputs
	PrivateKey types.String `tfsdk:"private_key"`
	User       types.String `tfsdk:"user"`
	Host       types.String `tfsdk:"host"`
	K3sConfig  types.String `tfsdk:"config"`
	// Outputs
	Id         types.String `tfsdk:"id"`
	KubeConfig types.String `tfsdk:"kubeconfig"`
	Token      types.String `tfsdk:"token"`
	Active     types.Bool   `tfsdk:"active"`
}

func (s *ServerClientModel) sshClient() (ssh_client.SSHClient, error) {
	return ssh_client.NewSSHClient(fmt.Sprintf("%s:22", s.Host.ValueString()), s.User.ValueString(), s.PrivateKey.ValueString())
}

// Configure implements resource.ResourceWithConfigure.
func (s *K3sServerResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	// TODO: Fix
	// provider := req.ProviderData.(K3sProvider)
	// if provider.Version != "" {
	// 	s.version = &provider.Version
	// }
}

// Create implements resource.ResourceWithImportState.
func (s *K3sServerResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data ServerClientModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Let the k3sClient write the ssh outputs
	// to the terraform logs
	logger := func(out string) {
		tflog.Warn(ctx, out)
	}

	sshClient, err := data.sshClient()
	if err != nil {
		resp.Diagnostics.Append(fromError("Creating ssh config", err))
		return
	}
	var config map[string]any
	if err := yaml.Unmarshal([]byte(data.K3sConfig.ValueString()), &config); err != nil {
		resp.Diagnostics.Append(fromError("Creating k3s config", err))
		return
	}

	server := k3s.NewK3sServerComponent(config, s.version)

	if err := server.RunPreReqs(sshClient, logger); err != nil {
		resp.Diagnostics.Append(fromError("Running k3s server prereqs", err))
		return
	}

	if err := server.RunInstall(sshClient, logger); err != nil {
		resp.Diagnostics.Append(fromError("Running k3s server prereqs", err))
		return
	}

	active, err := server.Status(sshClient)
	if err != nil {
		resp.Diagnostics.Append(fromError("Error retrieving server status", err))
		return
	}

	// Set outputs
	data.Active = types.BoolValue(active)
	data.KubeConfig = types.StringValue(server.KubeConfig())
	data.Token = types.StringValue(server.Token())
	id, _ := uuid.GenerateUUID()
	data.Id = types.StringValue(id)

	tflog.Warn(ctx, "created a k3s server resource")

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

	// Let the k3sClient write the ssh outputs
	// to the terraform logs
	logger := func(out string) {
		tflog.Warn(ctx, out)
	}

	sshClient, err := data.sshClient()
	if err != nil {
		resp.Diagnostics.Append(fromError("Creating ssh config", err))
		return
	}

	server := k3s.NewK3sServerComponent(nil, nil)
	if err := server.RunUninstall(sshClient, logger); err != nil {
		resp.Diagnostics.Append(fromError("Creating uninstall k3s", err))
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

	// k3sClient, err := NewK3sClient(data)
	// if err != nil {
	// 	resp.Diagnostics.Append(fromError("Creating ssh config", err))
	// 	return
	// }
	// active, err := k3sClient.IsActive()
	// if err != nil {
	// 	resp.Diagnostics.Append(fromError("Error retrieving server status", err))
	// 	return
	// }
	// data.Active = types.BoolValue(active && false)

	// resp.Diagnostics.Append(req.State.Set(ctx, &data)...)
}

// Schema implements resource.ResourceWithImportState.
func (s *K3sServerResource) Schema(context context.Context, resource resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: `Creates a K3s Server
Example:
` + TfMd(`
data "k3s_config" "server" {
  data_dir = "/etc/k3s"
  config  = {
	  "etcd-expose-metrics" = "" // flag for true
	  "etcd-s3-timeout"     = "5m30s",
	  "node-label"		    = ["foo=bar"]
  }
}

resource "k3s_server" "main" {
  host        = "192.168.10.1"
  user        = "ubuntu"
  private_key = var.private_key_openssh
  config      = data.k3s_server_config.server.yaml
}`),

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
			"config": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "K3s server config",
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
