package provider

import (
	"context"
	"fmt"
	"maps"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int32default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/mapplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"striveworks.us/terraform-provider-k3s/internal/k3s"
	"striveworks.us/terraform-provider-k3s/internal/ssh_client"
)

var _ resource.ResourceWithConfigValidators = &K3sHaServerResource{}

type K3sHaServerResource struct {
	version *string
}

func NewK3sHaServerResource() resource.Resource {
	return &K3sHaServerResource{}
}

type HaServerNodeModel struct {
	NodeAuth
	// Connection
	Host types.String `tfsdk:"host"`
	Port types.Int32  `tfsdk:"port"`
	// Configs
	BinDir types.String `tfsdk:"bin_dir"`
	TlsSan types.String `tfsdk:"tls_san"`
}

func (s *HaServerNodeModel) sshClient(ctx context.Context, globals NodeAuth) (ssh_client.SSHClient, error) {
	user := ""
	if !s.User.IsNull() {
		user = s.User.ValueString()
	} else {
		user = globals.User.ValueString()
	}

	pem := ""
	if !s.PrivateKey.IsNull() {
		pem = s.PrivateKey.ValueString()
	} else if !globals.PrivateKey.IsNull() {
		pem = globals.PrivateKey.ValueString()
	}

	password := ""
	if !s.Password.IsNull() {
		password = s.Password.ValueString()
	} else if !globals.Password.IsNull() {
		password = globals.Password.ValueString()
	}

	port := s.Port.ValueInt32()
	if port == 0 {
		port = 22
	}

	return ssh_client.NewSSHClient(
		ctx, fmt.Sprintf("%s:%d", s.Host.ValueString(), port), user, pem, password,
	)
}

var ModelAttrs = map[string]attr.Type{
	"host":        types.StringType,
	"port":        types.Int32Type,
	"private_key": types.StringType,
	"password":    types.StringType,
	"user":        types.StringType,
	"tls_san":     types.StringType,
	"bin_dir":     types.StringType,
}

type HaServerClientModel struct {
	NodeAuth
	// Configs
	BinDir      types.String `tfsdk:"bin_dir"`
	K3sConfig   types.String `tfsdk:"config"`
	K3sRegistry types.String `tfsdk:"registry"`
	// Node config
	Nodes types.List `tfsdk:"node"`
	// Outputs
	Id         types.String `tfsdk:"id"`
	KubeConfig types.String `tfsdk:"kubeconfig"`
	Token      types.String `tfsdk:"token"`
	Active     types.Map    `tfsdk:"active"`
}

func (h *HaServerClientModel) parseNodes(ctx context.Context) (diag.Diagnostics, []HaServerNodeModel) {
	nodes := make([]HaServerNodeModel, 0, len(h.Nodes.Elements()))

	diag := h.Nodes.ElementsAs(ctx, &nodes, false)
	if diag.ErrorsCount() > 0 {
		return diag, []HaServerNodeModel{}
	}

	var nodesOut []attr.Value
	for _, node := range nodes {
		nodesOut = append(nodesOut, types.ObjectValueMust(ModelAttrs, map[string]attr.Value{
			"host": node.Host,
			"port": func() attr.Value {
				if node.Port.IsNull() {
					return types.Int32Value(22)
				} else {
					return node.Port
				}
			}(),
			"private_key": func() attr.Value {
				if node.PrivateKey.IsNull() {
					return h.PrivateKey
				} else {
					return node.PrivateKey
				}
			}(),
			"password": func() attr.Value {
				if node.Password.IsNull() {
					return h.Password
				} else {
					return node.Password
				}
			}(),
			"user": func() attr.Value {
				if node.User.IsNull() {
					return h.User
				} else {
					return node.User
				}
			}(),
			"tls_san": node.TlsSan,
			"bin_dir": func() attr.Value {
				if node.BinDir.IsNull() {
					return types.StringValue("/usr/local/bin")
				} else {
					return node.BinDir
				}
			}(),
		}))
	}

	h.Nodes = types.ListValueMust(types.ObjectType{AttrTypes: ModelAttrs}, nodesOut)
	// Set again with new defaults
	diag = h.Nodes.ElementsAs(ctx, &nodes, false)
	return diag, nodes
}

func (s *K3sHaServerResource) description() MarkdownDescription {
	return `
Creates a K3s Highly Available Server

Example:

!!!hcl
resource "k3s_ha_server" "main" {
  user        = "ubuntu"
  private_key = var.ssh_key

  node {
    host = var.nodes[0]
  }
  node {
    host = var.nodes[1]
  }
  node {
    host = var.nodes[2]
  }
}
!!!
`
}

// Schema implements resource.ResourceWithImportState.
func (h *K3sHaServerResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: h.description().ToMarkdown(),
		Attributes: map[string]schema.Attribute{
			// Auth
			"private_key": schema.StringAttribute{
				Optional:            true,
				Sensitive:           true,
				MarkdownDescription: "Value of a privatekey used to auth, can be overridden in each node block",
			},
			"password": schema.StringAttribute{
				Optional:            true,
				Sensitive:           true,
				MarkdownDescription: "Username of the target server, can be overridden in each node block",
			},
			"user": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Username of the target server, can be overridden in each node block",
			},
			// Config
			"bin_dir": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Value of a path used to put the k3s binary, can be overridden in each node block",
				Default:             stringdefault.StaticString("/usr/local/bin"),
				Computed:            true,
			},
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
				MarkdownDescription: "Id of the k3s ha server resource",
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
				Sensitive: true,
			},
			"token": schema.StringAttribute{
				Computed:            true,
				Sensitive:           true,
				MarkdownDescription: "Server token used for joining nodes to the cluster",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"active": schema.MapAttribute{
				Computed:            true,
				MarkdownDescription: "The health of each server",
				ElementType:         types.BoolType,
				PlanModifiers: []planmodifier.Map{
					mapplanmodifier.UseStateForUnknown(),
				},
			},
		},
		Blocks: map[string]schema.Block{
			"node": schema.ListNestedBlock{
				Description: "Configuration for each server node",
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						// Connection
						"host": schema.StringAttribute{
							Required:            true,
							MarkdownDescription: "Hostname of the target server",
						},
						"port": schema.Int32Attribute{
							Optional:            true,
							MarkdownDescription: "Override default SSH port (22)",
							Default:             int32default.StaticInt32(22),
							Computed:            true,
						},
						// Auth
						"private_key": schema.StringAttribute{
							Optional:            true,
							Sensitive:           true,
							MarkdownDescription: "Value of a privatekey used to auth",
							Computed:            true,
						},
						"password": schema.StringAttribute{
							Optional:            true,
							Sensitive:           true,
							MarkdownDescription: "Username of the target server",
							Computed:            true,
						},
						"user": schema.StringAttribute{
							Optional:            true,
							MarkdownDescription: "Username of the target server",
							Computed:            true,
						},
						// Config
						"tls_san": schema.StringAttribute{
							Optional:            true,
							MarkdownDescription: "Fixed IP to be set on the server",
						},
						"bin_dir": schema.StringAttribute{
							Optional:            true,
							MarkdownDescription: "Value of a path used to put the k3s binary",
							Default:             stringdefault.StaticString("/usr/local/bin"),
							Computed:            true,
						},
					},
				},
			},
		},
	}
}

// Configure implements resource.ResourceWithConfigure.
func (h *K3sHaServerResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	provider, ok := req.ProviderData.(*K3sProvider)
	if !ok {
		resp.Diagnostics.AddError("Could not convert provider data into version", "")
		return
	}
	if provider.Version != "" {
		h.version = &provider.Version
	}
}

func (s *K3sHaServerResource) ConfigValidators(context.Context) []resource.ConfigValidator {
	return []resource.ConfigValidator{
		NewK3sHaServerValidator(),
	}
}

// Create implements resource.ResourceWithConfigValidators.
func (s *K3sHaServerResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data HaServerClientModel
	activeMap := make(map[string]attr.Value)

	// Read Terraform state data into the model
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Read node block data into the model
	diag, nodes := data.parseNodes(ctx)
	resp.Diagnostics.Append(diag...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Info(ctx, "Creating ssh clients")
	// Build each node's ssh client
	var sshClients []ssh_client.SSHClient
	for _, node := range nodes {
		client, err := node.sshClient(ctx, data.NodeAuth)
		if err != nil {
			resp.Diagnostics.AddError("Building ssh clients", fmt.Sprintf("Could not create ssh client: %v", err.Error()))
		}
		sshClients = append(sshClients, client)
	}

	if resp.Diagnostics.HasError() {
		return
	}

	config, registry, err := setupConfiguration(ctx, data)
	if err != nil {
		resp.Diagnostics.AddError("Creating k3s registry", err.Error())
		return
	}

	// First Node
	firstConfig := maps.Clone(config)
	api, err := s.configureFirstNode(ctx, sshClients[0], registry, firstConfig, data.BinDir.ValueString(), nodes[0])
	if err != nil {
		resp.Diagnostics.AddError("Creating first server", err.Error())
		return
	}
	activeMap[nodes[0].Host.ValueString()] = types.BoolValue(true)

	nodeConfig := maps.Clone(config)
	nodeConfig["server"] = fmt.Sprintf("https://%s:6443", api.host)
	nodeConfig["token"] = api.token

	errs := make(chan error, len(nodes)-1)
	done := make(chan struct{}, len(nodes)-1)

	for i := 1; i < len(nodes); i++ {
		go func(idx int) {
			defer func() { done <- struct{}{} }()
			node := nodes[idx]
			if _, err := s.configureNode(ctx, sshClients[idx], registry, nodeConfig, node.BinDir.ValueString(), node); err != nil {
				activeMap[nodes[idx].Host.ValueString()] = types.BoolValue(false)
				errs <- fmt.Errorf("error configuring node %s: %w", node.Host.ValueString(), err)
				return
			}
			activeMap[node.Host.ValueString()] = types.BoolValue(true)
		}(i)
	}

	// Wait for all goroutines to finish
	for i := 1; i < len(nodes)-1; i++ {
		<-done
	}
	close(done)

	// Collect errors if any
	for i := 1; i < len(nodes); i++ {
		select {
		case err := <-errs:
			if err != nil {
				resp.Diagnostics.AddError("Configuring server node", err.Error())
			}
		default:
			// No error to collect
		}
	}
	close(errs)

	if resp.Diagnostics.HasError() {
		return
	}

	if data.BinDir.IsNull() {
		data.BinDir = types.StringValue("/usr/local/bin")
	}

	data.Token = types.StringValue(api.token)
	data.Active, _ = types.MapValue(types.BoolType, activeMap)
	data.Id = types.StringValue(api.token)
	data.KubeConfig = types.StringValue(api.kubeconfig)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)

}

// Delete implements resource.ResourceWithConfigValidators.
func (s *K3sHaServerResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data HaServerClientModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Read node block data into the model
	diag, nodes := data.parseNodes(ctx)
	resp.Diagnostics.Append(diag...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Build each node's ssh client
	var sshClients []ssh_client.SSHClient
	for _, node := range nodes {
		client, err := node.sshClient(ctx, data.NodeAuth)
		if err != nil {
			resp.Diagnostics.AddError("Building ssh clients", fmt.Sprintf("Could not create ssh client: %v", err.Error()))
		}
		sshClients = append(sshClients, client)
	}

	errs := make(chan error, len(nodes)-1)
	done := make(chan struct{}, len(nodes)-1)

	for i := range nodes {
		go func(idx int) {
			defer func() { done <- struct{}{} }()
			node := nodes[idx]

			if err := k3s.NewK3sServerComponent(
				ctx, nil, nil, s.version, node.BinDir.ValueString(),
			).RunUninstall(sshClients[idx]); err != nil {
				errs <- fmt.Errorf("error configuring node %s: %w", node.Host.ValueString(), err)
				return
			}

		}(i)
	}

	// Wait for all goroutines to finish
	for range nodes {
		<-done
	}
	close(done)
	// Collect errors if any
	for range nodes {
		select {
		case err := <-errs:
			if err != nil {
				resp.Diagnostics.AddError("Configuring server node", err.Error())
			}
		default:
			// No error to collect
		}
	}
	close(errs)

	resp.Diagnostics.Append(req.State.Set(ctx, &data)...)
}

// Metadata implements resource.ResourceWithConfigValidators.
func (s *K3sHaServerResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_ha_server"
}

// Read implements resource.ResourceWithConfigValidators.
func (s *K3sHaServerResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data HaServerClientModel
	activeMap := make(map[string]attr.Value)

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Read node block data into the model
	diag, nodes := data.parseNodes(ctx)
	resp.Diagnostics.Append(diag...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Build each node's ssh client
	var sshClients []ssh_client.SSHClient
	for _, node := range nodes {
		client, err := node.sshClient(ctx, data.NodeAuth)
		if err != nil {
			resp.Diagnostics.AddError("Building ssh clients", fmt.Sprintf("Could not create ssh client: %v", err.Error()))
		}
		sshClients = append(sshClients, client)
	}

	errs := make(chan error, len(nodes)-1)
	done := make(chan struct{}, len(nodes)-1)

	for i := range nodes {
		go func(idx int) {
			defer func() { done <- struct{}{} }()
			node := nodes[idx]

			status, err := k3s.NewK3sServerComponent(
				ctx, nil, nil, s.version, node.BinDir.ValueString(),
			).Status(sshClients[idx])

			if err != nil {
				activeMap[nodes[idx].Host.ValueString()] = types.BoolValue(false)
				errs <- fmt.Errorf("error configuring node %s: %w", node.Host.ValueString(), err)
				return
			}

			activeMap[node.Host.ValueString()] = types.BoolValue(status)
		}(i)
	}

	// Wait for all goroutines to finish
	for range nodes {
		<-done
	}
	close(done)
	// Collect errors if any
	for range nodes {
		select {
		case err := <-errs:
			if err != nil {
				resp.Diagnostics.AddError("Configuring server node", err.Error())
			}
		default:
			// No error to collect
		}
	}
	close(errs)

	data.Active, _ = types.MapValue(types.BoolType, activeMap)
	resp.Diagnostics.Append(req.State.Set(ctx, &data)...)

}

// Update implements resource.ResourceWithConfigValidators.
func (s *K3sHaServerResource) Update(context.Context, resource.UpdateRequest, *resource.UpdateResponse) {

}

func setupConfiguration(ctx context.Context, data HaServerClientModel) (map[string]any, map[string]any, error) {
	config, err := ParseK3sConfig(&data.K3sConfig)
	if err != nil {

		return nil, nil, err
	}
	tflog.Debug(ctx, "K3s config parsed")

	registry, err := ParseK3sRegistry(&data.K3sRegistry)
	if err != nil {
		return nil, nil, err
	}
	config["embedded-registry"] = registry != nil
	tflog.Debug(ctx, "K3s registry parsed")

	return config, registry, nil
}

type ApiSpec struct {
	kubeconfig string
	token      string
	host       string
}

func (s *K3sHaServerResource) configureNode(
	ctx context.Context,
	sshClient ssh_client.SSHClient,
	registry map[string]any,
	config map[string]any,
	binDir string,
	node HaServerNodeModel,
) (ApiSpec, error) {
	if !node.TlsSan.IsNull() {
		config["tls-san"] = node.TlsSan.ValueString()
	}

	server := k3s.NewK3sServerComponent(
		ctx, config, registry, s.version, binDir,
	)

	tflog.Info(ctx, "Running k3s server preq steps on first server")
	if err := server.RunPreReqs(sshClient); err != nil {
		return ApiSpec{}, fmt.Errorf("running k3s server prereqs on first server: %v", err.Error())

	}

	tflog.Info(ctx, "Running k3s server install on first server")
	if err := server.RunInstall(sshClient); err != nil {
		return ApiSpec{}, fmt.Errorf("running k3s server install on first server: %v", err.Error())

	}

	tflog.Info(ctx, "Checking k3s systemd status on first server")
	active, err := server.Status(sshClient)
	if err != nil {
		return ApiSpec{}, fmt.Errorf("retrieving server status on first server: %v", err.Error())
	}

	if !active {
		status, err := server.StatusLog(sshClient)
		if err != nil {
			return ApiSpec{}, fmt.Errorf("retrieving systemctl status on first server: %v", err.Error())
		}
		tflog.Debug(ctx, status)

		logs, err := server.Journal(sshClient)
		if err != nil {
			return ApiSpec{}, fmt.Errorf("retrieving journalctl status on first server: %v", err.Error())
		}
		tflog.Trace(ctx, logs)

		return ApiSpec{}, fmt.Errorf("was not able to activate first server. Check TRACE logs for provider to see journalctl dump")
	}

	return ApiSpec{token: server.Token(), kubeconfig: server.KubeConfig(), host: node.Host.ValueString()}, nil
}

// Configures the first node in the HA server setup. This will configure the token and bootstrap the node with it.
func (s *K3sHaServerResource) configureFirstNode(ctx context.Context, sshClient ssh_client.SSHClient, registry map[string]any, config map[string]any, binDir string, node HaServerNodeModel) (ApiSpec, error) {
	config["cluster-init"] = true
	return s.configureNode(ctx, sshClient, registry, config, binDir, node)
}
