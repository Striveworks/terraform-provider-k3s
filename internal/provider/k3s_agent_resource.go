package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/go-uuid"
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
	*NodeAuth
	// Connection
	Host   types.String `tfsdk:"host"`
	Server types.String `tfsdk:"server"`
	Port   types.Int32  `tfsdk:"port"`
	BinDir types.String `tfsdk:"bin_dir"`
	// Configs
	K3sConfig types.String `tfsdk:"config"`
	Token     types.String `tfsdk:"token"`
	// Outputs
	Id     types.String `tfsdk:"id"`
	Active types.Bool   `tfsdk:"active"`
}

func (K3sAgentResource) description() MarkdownDescription {
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
!!!
`
}

func (s *AgentClientModel) sshClient(ctx context.Context) (ssh_client.SSHClient, error) {
	port := 22
	if int(s.Port.ValueInt32()) != 0 {
		port = int(s.Port.ValueInt32())
	}

	addr := fmt.Sprintf("%s:%d", s.Host.ValueString(), port)
	return ssh_client.NewSSHClient(ctx, addr, s.User.ValueString(), s.PrivateKey.ValueString(), s.Password.ValueString())
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

	sshClient, err := data.sshClient(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Creating ssh config", err.Error())
		return
	}

	token := data.Token.ValueString()
	server := fmt.Sprintf("https://%s:6443", data.Server.ValueString())

	config := make(map[string]any)
	if err := yaml.Unmarshal([]byte(data.K3sConfig.ValueString()), &config); err != nil {
		resp.Diagnostics.AddError("Creating k3s config", err.Error())
		return
	}

	config["token"] = token
	config["server"] = server

	agent := k3s.NewK3sAgentComponent(ctx, config, k.version, data.BinDir.ValueString())
	if err := agent.RunPreReqs(sshClient); err != nil {
		resp.Diagnostics.AddError("Running k3s agent prereqs", err.Error())
		return
	}

	if err := agent.RunInstall(sshClient); err != nil {
		resp.Diagnostics.AddError("Running k3s agent install", err.Error())
		return
	}

	active, err := agent.Status(sshClient)
	if err != nil {
		resp.Diagnostics.AddError("Error retrieving agent status", err.Error())
		return
	}

	if !active {
		status, err := agent.StatusLog(sshClient)
		if err != nil {
			resp.Diagnostics.AddError("Error retrieving systemctl status", err.Error())
			return
		}
		resp.Diagnostics.AddError("Error running k3s-agent systemctl status", status)

		logs, err := agent.Journal(sshClient)
		if err != nil {
			resp.Diagnostics.AddError("Error retrieving journalctl status", err.Error())
			return
		}
		tflog.Trace(ctx, logs)
	}

	data.Active = types.BoolValue(active)
	id, _ := uuid.GenerateUUID()
	data.Id = types.StringValue(id)

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

	agent := k3s.NewK3sAgentComponent(ctx, nil, nil, data.BinDir.ValueString())
	if err := agent.RunUninstall(sshClient); err != nil {
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
	agent := k3s.NewK3sAgentComponent(ctx, nil, k.version, data.BinDir.ValueString())

	active, err := agent.Status(sshClient)
	if err != nil {
		resp.Diagnostics.AddError("Error retrieving agent status", err.Error())
		return
	}
	data.Active = types.BoolValue(active)

	resp.Diagnostics.Append(req.State.Set(ctx, &data)...)
}

// Schema implements resource.Resource.
func (k *K3sAgentResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: k.description().ToMarkdown(),

		Attributes: map[string]schema.Attribute{
			// Inputs
			"bin_dir": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Value of a path used to put the k3s binary",
				Default:             stringdefault.StaticString("/usr/local/bin"),
				Computed:            true,
			},
			// Auth
			"private_key": schema.StringAttribute{
				Sensitive:           true,
				Optional:            true,
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
			// Conn
			"host": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Hostname of the target server",
			},
			"port": schema.Int32Attribute{
				Optional:            true,
				MarkdownDescription: "Override default SSH port (22)",
			},
			// Config
			"config": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "K3s server config",
			},
			"token": schema.StringAttribute{
				Required:            true,
				Sensitive:           true,
				MarkdownDescription: "Server token used for joining nodes to the cluster",
			},
			"server": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "K3s server address",
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

	agent := k3s.NewK3sAgentComponent(ctx, nil, nil, data.BinDir.ValueString())
	if err := agent.RunUninstall(sshClient); err != nil {
		resp.Diagnostics.AddError("Creating uninstall k3s", err.Error())
		return
	}
}

// ConfigValidators implements resource.ResourceWithConfigValidators.
func (s *K3sAgentResource) ConfigValidators(ctx context.Context) []resource.ConfigValidator {
	return []resource.ConfigValidator{
		&k3sAgentAuthValdiator{},
	}
}

func NewK3sAgentResource() resource.Resource {
	return &K3sAgentResource{}
}
