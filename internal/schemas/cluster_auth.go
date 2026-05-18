package schemas

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	datasourceschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	resourceschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/objectplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"k8s.io/client-go/tools/clientcmd"
	api "k8s.io/client-go/tools/clientcmd/api"
)

// ClusterAuth represents the authentication details required to connect to a
// K3s Kubernetes cluster. It encapsulates various certificate and key data,
// along with the API server endpoint, necessary for client authentication.
type ClusterAuth struct {
	ClientCertificateData    types.String `tfsdk:"client_certificate_data"`
	CertificateAuthorityData types.String `tfsdk:"certificate_authority_data"`
	ClientKeyData            types.String `tfsdk:"client_key_data"`
	Server                   types.String `tfsdk:"server"`

	config *api.Config
}

// DefaultK3sClusterAuth returns a default, null-initialized basetypes.ObjectValue
// for the ClusterAuth schema. This is useful for providing a default empty
// configuration when the cluster authentication details are not yet known.
func DefaultK3sClusterAuth() basetypes.ObjectValue {
	return types.ObjectNull(ClusterAuth{}.AttributeTypes())
}

// AttributeTypes returns a map of attribute names to their corresponding
// Terraform attribute types (`attr.Type`) for the ClusterAuth struct.
// This map is used by the Terraform plugin framework for type conversion.
func (m ClusterAuth) AttributeTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"client_certificate_data":    types.StringType,
		"certificate_authority_data": types.StringType,
		"client_key_data":            types.StringType,
		"server":                     types.StringType,
	}
}

// Validate performs any custom validation checks on the ClusterAuth object.
// Currently, no specific validation rules are enforced at this level.
func (n ClusterAuth) Validate() error {
	return nil
}

// ToObject converts the ClusterAuth instance into a `basetypes.ObjectValue`.
// This is used for serialization within the Terraform plugin framework.
func (m *ClusterAuth) ToObject(ctx context.Context) basetypes.ObjectValue {
	return ToObject(ctx, m)
}

// Schema returns the Terraform schema definition for the ClusterAuth object.
// It defines the structure and types of the cluster authentication attributes
// exposed to Terraform configurations.
func (n ClusterAuth) Schema() resourceschema.Attribute {
	return resourceschema.SingleNestedAttribute{
		Computed:    true,
		Description: "Cluster authentication details for connecting to the K3s cluster.",
		PlanModifiers: []planmodifier.Object{
			objectplanmodifier.UseStateForUnknown(),
		},
		Attributes: map[string]resourceschema.Attribute{
			"client_certificate_data": resourceschema.StringAttribute{
				Computed:            true,
				Sensitive:           true,
				MarkdownDescription: "Base64 encoded client certificate data for authenticating to the Kubernetes API server.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"certificate_authority_data": resourceschema.StringAttribute{
				Computed:            true,
				Sensitive:           true,
				MarkdownDescription: "Base64 encoded certificate authority data for authenticating to the Kubernetes API server.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"client_key_data": resourceschema.StringAttribute{
				Computed:            true,
				Sensitive:           true,
				MarkdownDescription: "Base64 encoded client key data for authenticating to the Kubernetes API server.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"server": resourceschema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The URL of the Kubernetes API server endpoint.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}

func (n ClusterAuth) DataSourceSchema() datasourceschema.Attribute {
	return datasourceschema.SingleNestedAttribute{
		Computed:    true,
		Description: "Cluster authentication details for connecting to the K3s cluster.",
		Attributes: map[string]datasourceschema.Attribute{
			"client_certificate_data": datasourceschema.StringAttribute{
				Computed:            true,
				Sensitive:           true,
				MarkdownDescription: "Base64 encoded client certificate data for authenticating to the Kubernetes API server.",
			},
			"certificate_authority_data": datasourceschema.StringAttribute{
				Computed:            true,
				Sensitive:           true,
				MarkdownDescription: "Base64 encoded certificate authority data for authenticating to the Kubernetes API server.",
			},
			"client_key_data": datasourceschema.StringAttribute{
				Computed:            true,
				Sensitive:           true,
				MarkdownDescription: "Base64 encoded client key data for authenticating to the Kubernetes API server.",
			},
			"server": datasourceschema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The URL of the Kubernetes API server endpoint.",
			},
		},
	}
}

// BuildClusterAuth constructs a ClusterAuth object from a raw Kubeconfig string.
// It parses the Kubeconfig, extracts relevant authentication and cluster details,
// and populates the ClusterAuth struct.
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

// UpdateHost modifies the server address within the embedded Kubernetes
// client configuration (`c.config`) and updates the `Server` field of the
// ClusterAuth struct.
func (c *ClusterAuth) UpdateHost(newHost string) {
	this := *c.config.Clusters["default"]
	this.Server = fmt.Sprintf("https://%s:6443", newHost)
	c.config.Clusters["default"] = &this

	c.Server = types.StringValue(c.config.Clusters["default"].Server)
}

// KubeConfig returns the complete Kubeconfig string derived from the
// internal Kubernetes client configuration. This string can be used to
// interact with the K3s cluster using standard Kubernetes tools.
func (c *ClusterAuth) KubeConfig() string {
	kubeconfig, _ := clientcmd.Write(*c.config)
	return string(kubeconfig)
}
