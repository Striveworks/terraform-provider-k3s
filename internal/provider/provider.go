package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
)

var (
	_ provider.Provider = &K3sProvider{}
)

// New is a helper function to simplify provider server and testing implementation.
func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &K3sProvider{}
	}
}

func NewDebugMode(version string) func() provider.Provider {
	return func() provider.Provider {
		return &K3sProvider{
			DebugMode: true,
		}
	}
}

type K3sProvider struct {
	DebugMode bool
}

type k3sProviderModel struct{}

// Metadata returns the provider type name.
func (p *K3sProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "k3s"
}

// Schema defines the provider-level schema for configuration data.
func (p *K3sProvider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: ("K3s Terraform Provider\n" +
			"Use with your favorite cloud provider, openstack or baremetal. Makes no assumptions about the target backend."),
		Attributes: map[string]schema.Attribute{},
	}
}

// Configure prepares a HashiCups API client for data sources and resources.
func (p *K3sProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var config k3sProviderModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.ResourceData = &K3sProvider{}
	resp.DataSourceData = &K3sProvider{}
}

// DataSources defines the data sources implemented in the provider.
func (p *K3sProvider) DataSources(_ context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		NewK3sKubeConfigData,
	}
}

// Resources defines the resources implemented in the provider.
func (p *K3sProvider) Resources(_ context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewK3sServerResource,
		NewK3sAgentResource,
	}
}
