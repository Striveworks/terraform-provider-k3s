package provider

import (
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"gopkg.in/yaml.v2"
)

type MarkdownDescription string

func (s MarkdownDescription) ToMarkdown() string {
	return strings.ReplaceAll(string(s), "!!!", "```")
}

func ParseK3sRegistry(value *basetypes.StringValue) (registry map[string]any, err error) {
	if value.IsNull() {
		return
	}
	err = yaml.Unmarshal([]byte(value.ValueString()), &registry)
	return
}

func ParseK3sConfig(value *basetypes.StringValue) (config map[string]any, err error) {
	config, err = ParseK3sRegistry(value)
	if config == nil {
		config = make(map[string]any)
	}
	return
}

type NodeAuth struct {
	PrivateKey types.String `tfsdk:"private_key"`
	Password   types.String `tfsdk:"password"`
	User       types.String `tfsdk:"user"`
}
