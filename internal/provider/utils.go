package provider

import (
	"fmt"
	"maps"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"gopkg.in/yaml.v2"
	"k8s.io/client-go/tools/clientcmd"
	api "k8s.io/client-go/tools/clientcmd/api"
)

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

type ClusterAuth struct {
	ClientCertificateData    types.String `tfsdk:"client_certificate_data"`
	CertificateAuthorityData types.String `tfsdk:"certificate_authority_data"`
	ClientKeyData            types.String `tfsdk:"client_key_data"`
	Server                   types.String `tfsdk:"server"`

	config *api.Config
}

func (m ClusterAuth) AttributeTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"client_certificate_data":    types.StringType,
		"certificate_authority_data": types.StringType,
		"client_key_data":            types.StringType,
		"server":                     types.StringType,
	}
}

func NewClusterAuth(kubeconfig string) (ClusterAuth, error) {
	data := ClusterAuth{}
	config, err := clientcmd.Load([]byte(kubeconfig))
	if err != nil {
		return data, err
	}

	data.config = config
	// Set host
	data.Server = types.StringValue(config.Clusters["default"].Server)
	// Set cluster CA
	data.CertificateAuthorityData = types.StringValue(string(config.Clusters["default"].CertificateAuthorityData))
	// Set User cert
	data.ClientCertificateData = types.StringValue(string(config.AuthInfos["default"].ClientCertificateData))
	// Set User Key
	data.ClientKeyData = types.StringValue(string(config.AuthInfos["default"].ClientKeyData))

	return data, nil
}
func (c *ClusterAuth) UpdateHost(newHost string) {
	this := *c.config.Clusters["default"]
	this.Server = fmt.Sprintf("https://%s:6443", newHost)
	c.config.Clusters["default"] = &this

	c.Server = types.StringValue(c.config.Clusters["default"].Server)
}

func (c *ClusterAuth) KubeConfig() string {
	kubeconfig, _ := clientcmd.Write(*c.config)
	return string(kubeconfig)
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
