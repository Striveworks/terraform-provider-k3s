package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"

	"striveworks.us/terraform-provider-k3s/internal/handlers"
	"striveworks.us/terraform-provider-k3s/internal/k3s"
)

type K3sKubeConfigData struct{}

func NewK3sKubeConfigData() datasource.DataSource {
	return &K3sKubeConfigData{}
}

// Metadata implements datasource.DataSource.
func (k *K3sKubeConfigData) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_kubeconfig"
}

// Read implements datasource.DataSource.
func (k *K3sKubeConfigData) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data handlers.K3sKubeConfig
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	auth := handlers.NewNodeAuth(ctx, data.Auth)
	server := k3s.NewK3ServerUninstall(ctx, "")
	if err := data.Read(ctx, &auth, server); err != nil {
		resp.Diagnostics.AddError("error reading kubeconfig", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// Schema implements datasource.DataSource.
func (k *K3sKubeConfigData) Schema(_ context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: ("A utility for reading and manipulating kubeconfig. Common use case would be to nicely extract " +
			"the auth credentials or overridding the server url for a load balancer url or dns name."),
		Attributes: map[string]schema.Attribute{
			"auth":         handlers.NodeAuth{}.Schema(),
			"cluster_auth": handlers.ClusterAuth{}.Schema(),
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
			"k3s_url": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "K3S_URL variable",
			},
		},
	}
}
