package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type K3sKubeConfigData struct{}

func NewK3sKubeConfigData() datasource.DataSource {
	return &K3sKubeConfigData{}
}

type K3sKubeConfig struct {
	ClusterAuth
	KubeConfig types.String `tfsdk:"kubeconfig"`
	Hostname   types.String `tfsdk:"hostname"`
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

	authData, err := NewClusterAuth(data.KubeConfig.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("malformed kubeconfig", err.Error())
		return
	}

	// Set hostname
	if !data.Hostname.IsNull() {
		authData.UpdateHost(data.Hostname.ValueString())
	}

	data.ClusterAuth = authData
	data.KubeConfig = types.StringValue(authData.KubeConfig())
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// Schema implements datasource.DataSource.
func (k *K3sKubeConfigData) Schema(_ context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		DeprecationMessage: "Use k3s_server.cluster_auth",
		MarkdownDescription: ("A utility for reading and manipulating kubeconfig. Common use case would be to nicely extract " +
			"the auth credentials or overridding the server url for a load balancer url or dns name."),
		Attributes: map[string]schema.Attribute{
			"kubeconfig": schema.StringAttribute{
				Required:            true,
				Sensitive:           true,
				MarkdownDescription: "Output of the kubeconfig from a k3s_server resource",
			},
			"hostname": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Override the api server's hostname",
			},
			"server": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Api server's address",
			},
			"client_certificate_data": schema.StringAttribute{
				Computed:            true,
				Sensitive:           true,
				MarkdownDescription: "Client user certificate, already base64 decoded",
			},
			"certificate_authority_data": schema.StringAttribute{
				Computed:            true,
				Sensitive:           true,
				MarkdownDescription: "Client CA, already base64 decoded",
			},
			"client_key_data": schema.StringAttribute{
				Computed:            true,
				Sensitive:           true,
				MarkdownDescription: "Client user key, already base64 decoded",
			},
		},
	}
}
