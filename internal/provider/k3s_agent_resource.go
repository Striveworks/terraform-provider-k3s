// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"striveworks.us/terraform-provider-k3s/internal/ssh_client"
)

var _ resource.ResourceWithConfigure = &K3sAgentResource{}
var _ resource.ResourceWithImportState = &K3sAgentResource{}

type K3sAgentResource struct {
	version *string
}

// AgentClientModel describes the resource data model.
type AgentClientModel struct {
	// Inputs
	PrivateKey types.String `tfsdk:"private_key"`
	User       types.String `tfsdk:"user"`
	Host       types.String `tfsdk:"host"`
	K3sConfig  types.String `tfsdk:"config"`
	Token      types.String `tfsdk:"token"`
	// Outputs
	Id     types.String `tfsdk:"id"`
	Active types.Bool   `tfsdk:"active"`
}

func (s *AgentClientModel) sshClient() (ssh_client.SSHClient, error) {
	return ssh_client.NewSSHClient(fmt.Sprintf("%s:22", s.Host.ValueString()), s.User.ValueString(), s.PrivateKey.ValueString())
}

// Configure implements resource.ResourceWithConfigure.
func (k *K3sAgentResource) Configure(context.Context, resource.ConfigureRequest, *resource.ConfigureResponse) {
	panic("unimplemented")
}

// ImportState implements resource.ResourceWithImportState.
func (k *K3sAgentResource) ImportState(context.Context, resource.ImportStateRequest, *resource.ImportStateResponse) {
	panic("unimplemented")
}

// Create implements resource.Resource.
func (k *K3sAgentResource) Create(context.Context, resource.CreateRequest, *resource.CreateResponse) {
	panic("unimplemented")
}

// Delete implements resource.Resource.
func (k *K3sAgentResource) Delete(context.Context, resource.DeleteRequest, *resource.DeleteResponse) {
	panic("unimplemented")
}

// Metadata implements resource.Resource.
func (k *K3sAgentResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_agent"
}

// Read implements resource.Resource.
func (k *K3sAgentResource) Read(context.Context, resource.ReadRequest, *resource.ReadResponse) {
	panic("unimplemented")
}

// Schema implements resource.Resource.
func (k *K3sAgentResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
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
}

resource "k3s_agent" "worker" {
  host        = "192.168.10.2"
  user        = "ubuntu"
  private_key = var.private_key_openssh
  config      = data.k3s_server_config.server.yaml
  token		  = k3s_server.main.token
}

`),

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

// Update implements resource.Resource.
func (k *K3sAgentResource) Update(context.Context, resource.UpdateRequest, *resource.UpdateResponse) {
	panic("unimplemented")
}

func NewK3sAgentResource() resource.Resource {
	return &K3sAgentResource{}
}
