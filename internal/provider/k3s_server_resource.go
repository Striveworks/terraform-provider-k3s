// Copyright (c) HashiCorp, Inc.

package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/go-uuid"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int32default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"striveworks.us/terraform-provider-k3s/internal/k3s"
	"striveworks.us/terraform-provider-k3s/internal/ssh_client"
)

var _ resource.ResourceWithConfigValidators = &K3sServerResource{}

type K3sServerResource struct {
	version *string
}

func NewK3sServerResource() resource.Resource {
	return &K3sServerResource{}
}

func (s *ServerClientModel) sshClient(ctx context.Context) (ssh_client.SSHClient, error) {
	port := 22
	if int(s.Port.ValueInt32()) != 0 {
		port = int(s.Port.ValueInt32())
	}

	addr := fmt.Sprintf("%s:%d", s.Host.ValueString(), port)
	return ssh_client.NewSSHClient(ctx, addr, s.User.ValueString(), s.PrivateKey.ValueString(), s.Password.ValueString())
}

// ServerClientModel describes the resource data model.
type ServerClientModel struct {
	NodeAuth
	// Connection
	Host types.String `tfsdk:"host"`
	Port types.Int32  `tfsdk:"port"`
	// Configs
	BinDir      types.String `tfsdk:"bin_dir"`
	K3sConfig   types.String `tfsdk:"config"`
	K3sRegistry types.String `tfsdk:"registry"`
	// Outputs
	Id         types.String `tfsdk:"id"`
	KubeConfig types.String `tfsdk:"kubeconfig"`
	Token      types.String `tfsdk:"token"`
	Active     types.Bool   `tfsdk:"active"`
}

func (s *K3sServerResource) description() MarkdownDescription {
	return `
Creates a K3s Server

Example:

!!!hcl
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
!!!
`
}

// Schema implements resource.ResourceWithImportState.
func (s *K3sServerResource) Schema(context context.Context, resource resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: s.description().ToMarkdown(),
		Attributes: map[string]schema.Attribute{
			// Inputs
			"bin_dir": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Value of a path used to put the k3s binary",
				Default:             stringdefault.StaticString("/usr/local/bin"),
				Computed:            true,
			},
			"private_key": schema.StringAttribute{
				Optional:            true,
				Sensitive:           true,
				MarkdownDescription: "Value of a privatekey used to auth",
			},
			"password": schema.StringAttribute{
				Optional:            true,
				Sensitive:           true,
				MarkdownDescription: "Username of the target server",
			},
			"user": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Username of the target server",
			},
			// Connection
			"host": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Hostname of the target server",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"port": schema.Int32Attribute{
				Optional:            true,
				Computed:            true,
				Default:             int32default.StaticInt32(22),
				MarkdownDescription: "Override default SSH port (22)",
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

// Configure implements resource.ResourceWithConfigure.
func (s *K3sServerResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	provider, ok := req.ProviderData.(*K3sProvider)
	if !ok {
		resp.Diagnostics.AddError("Could not convert provider data into version", "")
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

	sshClient, err := data.sshClient(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Creating ssh config", err.Error())
		return
	}
	tflog.Debug(ctx, "SSH client configured")

	config, err := ParseK3sConfig(&data.K3sConfig)
	if err != nil {
		resp.Diagnostics.AddError("Creating k3s config", err.Error())
		return
	}
	tflog.Debug(ctx, "K3s config parsed")

	registry, err := ParseK3sRegistry(&data.K3sRegistry)
	if err != nil {
		resp.Diagnostics.AddError("Creating k3s registry", err.Error())
		return
	}
	config["embedded-registry"] = registry != nil
	tflog.Debug(ctx, "K3s registry parsed")

	server := k3s.NewK3sServerComponent(ctx, config, registry, s.version, data.BinDir.ValueString())

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
	data.KubeConfig = types.StringValue(server.KubeConfig())
	data.Token = types.StringValue(server.Token())
	id, _ := uuid.GenerateUUID()
	data.Id = types.StringValue(id)

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

	sshClient, err := data.sshClient(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Creating ssh config", err.Error())
		return
	}

	server := k3s.NewK3sServerComponent(ctx, nil, nil, nil, data.BinDir.ValueString())
	if err := server.RunUninstall(sshClient); err != nil {
		resp.Diagnostics.AddError("Creating uninstall k3s", err.Error())
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

	sshClient, err := data.sshClient(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Creating ssh config", err.Error())
		return
	}

	server := k3s.NewK3sServerComponent(ctx, nil, nil, s.version, data.BinDir.ValueString())

	active, err := server.Status(sshClient)
	tflog.Info(ctx, fmt.Sprintf("Status after install %t", active))
	if err != nil {
		resp.Diagnostics.AddError("Error retrieving server status", err.Error())
		return
	}
	data.Active = types.BoolValue(active)

	resp.Diagnostics.Append(req.State.Set(ctx, &data)...)
}

// Update implements resource.ResourceWithImportState.
func (s *K3sServerResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data ServerClientModel
	var state ServerClientModel

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

	server := k3s.NewK3sServerComponent(ctx, nil, nil, nil, data.BinDir.ValueString())

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
	state.K3sRegistry = data.K3sRegistry
	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// ConfigValidators implements resource.ResourceWithConfigValidators.
func (s *K3sServerResource) ConfigValidators(ctx context.Context) []resource.ConfigValidator {
	return []resource.ConfigValidator{
		NewK3sServerValidator(),
	}
}
