// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

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

var _ resource.ResourceWithConfigure = &K3sAgentResource{}

type K3sAgentResource struct {
	version *string
}

// AgentClientModel describes the resource data model.
type AgentClientModel struct {
	// Inputs
	PrivateKey types.String `tfsdk:"private_key"`
	User       types.String `tfsdk:"user"`
	Host       types.String `tfsdk:"host"`
	Server     types.String `tfsdk:"server"`
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

}

// Create implements resource.Resource.
func (k *K3sAgentResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data AgentClientModel

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

	token := data.Token.ValueString()
	server := fmt.Sprintf("https://%s:6443", data.Server.ValueString())

	config := make(map[string]any)
	if err := yaml.Unmarshal([]byte(data.K3sConfig.ValueString()), &config); err != nil {
		resp.Diagnostics.Append(fromError("Creating k3s config", err))
		return
	}

	config["token"] = token
	config["server"] = server

	agent := k3s.NewK3sAgentComponent(config, k.version)

	if err := agent.RunPreReqs(sshClient, logger); err != nil {
		resp.Diagnostics.Append(fromError("Running k3s agent prereqs", err))
		return
	}

	if err := agent.RunInstall(sshClient, logger); err != nil {
		resp.Diagnostics.Append(fromError("Running k3s agent install", err))
		return
	}

	active, err := agent.Status(sshClient)
	if err != nil {
		resp.Diagnostics.Append(fromError("Error retrieving agent status", err))
		return
	}

	data.Active = types.BoolValue(active)
	id, _ := uuid.GenerateUUID()
	data.Id = types.StringValue(id)

	tflog.Warn(ctx, "created a k3s agent resource")

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

	agent := k3s.NewK3sAgentComponent(nil, nil)
	if err := agent.RunUninstall(sshClient, logger); err != nil {
		resp.Diagnostics.Append(fromError("Creating uninstall k3s-agent", err))
		return
	}
}

// Metadata implements resource.Resource.
func (k *K3sAgentResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_agent"
}

// Read implements resource.Resource.
func (k *K3sAgentResource) Read(context.Context, resource.ReadRequest, *resource.ReadResponse) {

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
  config      = data.k3s_config.server.yaml
}

resource "k3s_agent" "worker" {
  host        = "192.168.10.2"
  user        = "ubuntu"
  private_key = var.private_key_openssh
  config      = data.k3s_config.server.yaml
  server	  = "192.168.10.1"
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
			"server": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Server host used for joining nodes to the cluster",
			},
			"token": schema.StringAttribute{
				Required:            true,
				Sensitive:           true,
				MarkdownDescription: "Server token used for joining nodes to the cluster",
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

// Update implements resource.Resource.
func (k *K3sAgentResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
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

	agent := k3s.NewK3sAgentComponent(nil, nil)
	if err := agent.RunUninstall(sshClient, logger); err != nil {
		resp.Diagnostics.Append(fromError("Creating uninstall k3s", err))
		return
	}
}

func NewK3sAgentResource() resource.Resource {
	return &K3sAgentResource{}
}
