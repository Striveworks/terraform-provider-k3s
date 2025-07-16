package handlers

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/objectplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"k8s.io/client-go/tools/clientcmd"
	api "k8s.io/client-go/tools/clientcmd/api"
)

type ClusterAuth struct {
	ClientCertificateData    types.String `tfsdk:"client_certificate_data"`
	CertificateAuthorityData types.String `tfsdk:"certificate_authority_data"`
	ClientKeyData            types.String `tfsdk:"client_key_data"`
	Server                   types.String `tfsdk:"server"`

	config *api.Config
}

func DefaultK3sClusterAuth() basetypes.ObjectValue {
	return types.ObjectNull(ClusterAuth{}.AttributeTypes())
}

func (m ClusterAuth) AttributeTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"client_certificate_data":    types.StringType,
		"certificate_authority_data": types.StringType,
		"client_key_data":            types.StringType,
		"server":                     types.StringType,
	}
}

func (n ClusterAuth) Validate() error {
	return nil
}

func NewClusterAuth(ctx context.Context, t basetypes.ObjectValue) ClusterAuth {
	var na ClusterAuth
	t.As(ctx, &na, basetypes.ObjectAsOptions{})
	return na
}

func (m *ClusterAuth) ToObject(ctx context.Context) basetypes.ObjectValue {
	return ToObject(ctx, m)
}

func (n ClusterAuth) Schema() schema.Attribute {
	return schema.SingleNestedAttribute{
		Computed:    true,
		Description: "Cluster auth objects",
		PlanModifiers: []planmodifier.Object{
			objectplanmodifier.UseStateForUnknown(),
		},
		Attributes: map[string]schema.Attribute{
			"client_certificate_data": schema.StringAttribute{
				Computed:            true,
				Sensitive:           true,
				MarkdownDescription: "Client user certificate, already base64 decoded",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"certificate_authority_data": schema.StringAttribute{
				Computed:            true,
				Sensitive:           true,
				MarkdownDescription: "Client CA, already base64 decoded",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"client_key_data": schema.StringAttribute{
				Computed:            true,
				Sensitive:           true,
				MarkdownDescription: "Client user key, already base64 decoded",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"server": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Apiserver address",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}

func BuildClusterAuth(kubeconfig string) (ClusterAuth, error) {
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
