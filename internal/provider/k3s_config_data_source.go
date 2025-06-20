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

var _ datasource.DataSource = &K3sConfigDataSource{}

type K3sConfigDataSource struct {
	Config  types.String `tfsdk:"config"`
	DataDir types.String `tfsdk:"data_dir"`
	Yaml    types.String `tfsdk:"yaml"`
}

func NewK3sConfigDataSource() datasource.DataSource {
	return &K3sConfigDataSource{}
}

// Metadata implements datasource.DataSource.
func (k *K3sConfigDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_config"
}

// Read implements datasource.DataSource.
func (k *K3sConfigDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data K3sConfigDataSource

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	dataDir := DATA_DIR
	if !data.DataDir.IsNull() {
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

	data.Yaml = types.StringValue(string(yamlBytes))
	data.DataDir = types.StringValue(dataDir)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)

}

func (k *K3sConfigDataSource) description() MarkdownDescription {
	return `
K3s configuration. Read more [here](https://docs.k3s.io/cli/server).

Example:

!!!hcl
data "k3s_config" "server" {
  data_dir = "/etc/k3s"
  config  = yamlencode({
	  "etcd-expose-metrics" = "" // flag for true
	  "etcd-s3-timeout"     = "5m30s",
	  "node-label"		    = ["foo=bar"]
	})
}
!!!
`
}

// Schema implements datasource.DataSource.
func (k *K3sConfigDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: k.description().ToMarkdown(),
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
