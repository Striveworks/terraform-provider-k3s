package provider

import (
	"maps"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"gopkg.in/yaml.v2"
)

type MarkdownDescription string

func (s MarkdownDescription) ToMarkdown() string {
	return strings.ReplaceAll(string(s), "!!!", "```")
}

func ParseYamlString(value basetypes.StringValue, mergeWith ...basetypes.StringValue) (config map[any]any, err error) {
	all := []basetypes.StringValue{value}
	for _, cfg := range append(all, mergeWith...) {
		local := make(map[any]any)
		if err = yaml.Unmarshal([]byte(cfg.ValueString()), &local); err != nil {
			return
		}
		config = mergeMaps(config, local)
	}
	return
}

type NodeAuth struct {
	PrivateKey types.String `tfsdk:"private_key"`
	Password   types.String `tfsdk:"password"`
	User       types.String `tfsdk:"user"`
}

func mergeMaps(a, b map[interface{}]interface{}) map[interface{}]interface{} {
	out := make(map[interface{}]interface{}, len(a))
	maps.Copy(out, a)
	for k, v := range b {
		// If you use map[string]interface{}, ok is always false here.
		// Because yaml.Unmarshal will give you map[interface{}]interface{}.
		if v, ok := v.(map[interface{}]interface{}); ok {
			if bv, ok := out[k]; ok {
				if bv, ok := bv.(map[interface{}]interface{}); ok {
					out[k] = mergeMaps(bv, v)
					continue
				}
			}
		}
		out[k] = v
	}
	return out
}
