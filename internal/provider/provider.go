package provider

import (
	"context"
	"os"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ provider.Provider = &K3sProvider{}
)

// New is a helper function to simplify provider server and testing implementation.
func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &K3sProvider{
			Version: version,
		}
	}
}

func NewDebugMode(version string) func() provider.Provider {
	return func() provider.Provider {
		return &K3sProvider{
			Version:   version,
			DebugMode: true,
		}
	}
}

type K3sProvider struct {
	Version   string
	DebugMode bool
}

type k3sProviderModel struct {
	// K3s version to select, if not selected
	// will default to latest
	Version types.String `tfsdk:"k3s_version"`
}

// Metadata returns the provider type name.
func (p *K3sProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "k3s"
	resp.Version = p.Version
}

// Schema defines the provider-level schema for configuration data.
func (p *K3sProvider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: ("K3s Terraform Provider\n" +
			"Use with your favorite cloud provider, openstack or baremetal. Makes no assumptions about the target backend."),
		Attributes: map[string]schema.Attribute{
			"k3s_version": schema.StringAttribute{
				Optional:    true,
				Description: "K3s version to select, if not selected will default to latest",
			},
		},
	}
}

// Configure prepares a HashiCups API client for data sources and resources.
func (p *K3sProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var config k3sProviderModel
	diags := req.Config.Get(ctx, &config)
	resp.Diagnostics.Append(diags...)

	if resp.Diagnostics.HasError() {
		return
	}

	if config.Version.IsUnknown() {
		resp.Diagnostics.AddAttributeError(
			path.Root("host"),
			"Unknown K3s Version",
			"The provider cannot create the K3s API client as there is an unknown configuration value for the K3s Version. "+
				"Either target apply the source of the value first, set the value statically in the configuration, or use the K3S_VERSION environment variable.",
		)
	}

	if resp.Diagnostics.HasError() {
		return
	}

	version := os.Getenv("K3S_VERSION")
	if strings.ToLower(version) == "latest" {
		version = ""
	}

	// Even if env var set, take provider explicitly set
	if !config.Version.IsNull() {
		version = config.Version.ValueString()
	}

	p.Version = version

	if resp.Diagnostics.HasError() {
		return
	}

	resp.ResourceData = p
	resp.DataSourceData = p
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
