// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"gopkg.in/yaml.v2"
)

const DATA_DIR string = "/var/lib/rancher/k3s"
const CONFIG_DIR string = "/etc/rancher/k3s"

var _ datasource.DataSource = &K3sServerConfigDataSource{}

type K3sServerConfigDataSource struct {
	Config  types.String `tfsdk:"config"`
	DataDir types.String `tfsdk:"data_dir"`
	Yaml    types.String `tfsdk:"yaml"`
}

func NewK3sServerConfigDataSource() datasource.DataSource {
	return &K3sServerConfigDataSource{}
}

// Metadata implements datasource.DataSource.
func (k *K3sServerConfigDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_server_config"
}

// Read implements datasource.DataSource.
func (k *K3sServerConfigDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data K3sServerConfigDataSource

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	var dataDir = DATA_DIR
	if data.DataDir.IsNull() {
		dataDir = data.DataDir.ValueString()
	}

	var config map[string]any
	if data.Config.IsNull() {
		config = make(map[string]any)
	} else {
		if err := yaml.Unmarshal([]byte(data.Config.ValueString()), &config); err != nil {
			resp.Diagnostics.Append(fromError("Error parsing config", err))
			return
		}
	}

	config["data-dir"] = dataDir

	yamlBytes, err := yaml.Marshal(config)
	if err != nil {
		resp.Diagnostics.Append(fromError("Error marshalling k3s server config as yaml", err))
		return
	}

	k.Yaml = types.StringValue(string(yamlBytes))
}

// Schema implements datasource.DataSource.
func (k *K3sServerConfigDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "K3s Server configuration. Read more [here](https://docs.k3s.io/cli/server).\nExample:\n" + TfMd(`
data "k3s_server_config" "server" {
  data_dir = "/etc/k3s"
  config  = yamlencode({
	  "etcd-expose-metrics" = "" // flag for true
	  "etcd-s3-timeout"     = "5m30s",
	  "node-label"		    = ["foo=bar"]
	})
} 
`),
		Attributes: map[string]schema.Attribute{
			"config": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Yaml encoded string of the config",
			},
			"data_dir": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Where k3s stores its data. Defaults to /var/lib/rancher/k3s",
			},
			"yaml": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Yaml formatted k3s server config file",
			},
		},
	}
}
