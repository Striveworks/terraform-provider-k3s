package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"striveworks.us/terraform-provider-k3s/internal/k3s"
)

type K3sKubeConfigData struct{}

func NewK3sKubeConfigData() datasource.DataSource {
	return &K3sKubeConfigData{}
}

type K3sKubeConfig struct {
	Auth        types.Object `tfsdk:"auth"`
	ClusterAuth types.Object `tfsdk:"cluster_auth"`
	KubeConfig  types.String `tfsdk:"kubeconfig"`
	Hostname    types.String `tfsdk:"hostname"`
	AllowEmpty  types.Bool   `tfsdk:"allow_empty"`
}

// Metadata implements datasource.DataSource.
func (k *K3sKubeConfigData) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_kubeconfig"
}

// Read implements datasource.DataSource.
func (k *K3sKubeConfigData) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data K3sKubeConfig
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	auth := NewNodeAuth(ctx, data.Auth)
	sshClient, err := auth.SshClient(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Creating ssh config", err.Error())
		return
	}

	server := k3s.NewK3sServerComponent(ctx, nil, nil, nil, "")
	if err := server.Resync(sshClient); err != nil {
		if data.AllowEmpty.ValueBool() {
			tflog.Info(ctx, "Allowing empty kubeconfig, returning nulls")
			// Set nulls safely
			data.Auth = types.ObjectNull(NodeAuth{}.AttributeTypes())
			data.ClusterAuth = types.ObjectNull(ClusterAuth{}.AttributeTypes())
			data.KubeConfig = types.StringNull()

			resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
			return
		}

		resp.Diagnostics.AddError("Error resyncing server", err.Error())
		return
	}
	clusterAuth, err := BuildClusterAuth(server.KubeConfig())
	if err != nil {
		resp.Diagnostics.AddError("Parsing kubeconfig", err.Error())
		return
	}

	// Set hostname
	if !data.Hostname.IsNull() {
		clusterAuth.UpdateHost(data.Hostname.ValueString())
	}

	data.Auth = auth.ToObject(ctx)
	data.ClusterAuth = clusterAuth.ToObject(ctx)
	data.KubeConfig = types.StringValue(clusterAuth.KubeConfig())
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// Schema implements datasource.DataSource.
func (k *K3sKubeConfigData) Schema(_ context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: ("A utility for reading and manipulating kubeconfig. Common use case would be to nicely extract " +
			"the auth credentials or overridding the server url for a load balancer url or dns name."),
		Attributes: map[string]schema.Attribute{
			"auth":         NodeAuth{}.Schema(),
			"cluster_auth": ClusterAuth{}.Schema(),
			"allow_empty": schema.BoolAttribute{
				Optional:            true,
				MarkdownDescription: "If this is true, it will allow a missing kubeconfig and set null to all outputs",
			},
			"kubeconfig": schema.StringAttribute{
				Computed:            true,
				Sensitive:           true,
				MarkdownDescription: "Output of the kubeconfig from a k3s_server resource",
			},
			"hostname": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Override the api server's hostname",
			},
		},
	}
}
