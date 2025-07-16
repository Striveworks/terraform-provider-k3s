package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int32default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/objectplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	api "k8s.io/client-go/tools/clientcmd/api"
	"striveworks.us/terraform-provider-k3s/internal/ssh_client"

	"k8s.io/client-go/tools/clientcmd"
)

func ToObject(ctx context.Context, o K3sType) basetypes.ObjectValue {
	obj, _ := types.ObjectValueFrom(ctx, o.AttributeTypes(), o)
	return obj
}

type K3sType interface {
	Schema() schema.Attribute
	AttributeTypes() map[string]attr.Type
	ToObject(context.Context) basetypes.ObjectValue
	Validate() error
}

type NodeAuth struct {
	Host       types.String `tfsdk:"host"`
	Port       types.Int32  `tfsdk:"port"`
	PrivateKey types.String `tfsdk:"private_key"`
	Password   types.String `tfsdk:"password"`
	User       types.String `tfsdk:"user"`
}

var _ K3sType = &NodeAuth{}

func (n NodeAuth) Validate() error {
	if n.PrivateKey.IsNull() && n.Password.IsNull() {
		return fmt.Errorf("neither password nor private key was passed")
	}

	if !n.PrivateKey.IsNull() && !n.Password.IsNull() {
		return fmt.Errorf("both password and private key were passed, only pass one")
	}

	return nil
}

func (auth NodeAuth) SshClient(ctx context.Context) (ssh_client.SSHClient, error) {

	port := 22
	if int(auth.Port.ValueInt32()) != 0 {
		port = int(auth.Port.ValueInt32())
	}

	return ssh_client.NewSSHClient(ctx, auth.Host.ValueString(), port, auth.User.ValueString(), auth.PrivateKey.ValueString(), auth.Password.ValueString())

}

func NewNodeAuth(ctx context.Context, t basetypes.ObjectValue) NodeAuth {
	var na NodeAuth
	t.As(ctx, &na, basetypes.ObjectAsOptions{})
	return na
}

func (n *NodeAuth) ToObject(ctx context.Context) basetypes.ObjectValue {
	return ToObject(ctx, n)
}

func (NodeAuth) Schema() schema.Attribute {
	return schema.SingleNestedAttribute{
		Required:    true,
		Description: "Auth configuration for the node",
		PlanModifiers: []planmodifier.Object{
			objectplanmodifier.UseStateForUnknown(),
		},
		Attributes: map[string]schema.Attribute{
			"private_key": schema.StringAttribute{
				Optional:            true,
				Sensitive:           true,
				MarkdownDescription: "Private ssh key value to be used in place of a password",
			},
			"password": schema.StringAttribute{
				Optional:            true,
				Sensitive:           true,
				MarkdownDescription: "Username of the target server",
			},
			"user": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Username of the target server",
			},
			"host": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Hostname of the target server",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"port": schema.Int32Attribute{
				Optional:            true,
				Computed:            true,
				Default:             int32default.StaticInt32(22),
				MarkdownDescription: "Override default SSH port (22)",
			},
		},
	}
}

func (m NodeAuth) AttributeTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"host":        types.StringType,
		"port":        types.Int32Type,
		"private_key": types.StringType,
		"password":    types.StringType,
		"user":        types.StringType,
	}
}

type ClusterAuth struct {
	ClientCertificateData    types.String `tfsdk:"client_certificate_data"`
	CertificateAuthorityData types.String `tfsdk:"certificate_authority_data"`
	ClientKeyData            types.String `tfsdk:"client_key_data"`
	Server                   types.String `tfsdk:"server"`

	config *api.Config
}

var _ K3sType = &ClusterAuth{}

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

type OidcConfig struct {
	Audience     types.String `tfsdk:"audience"`
	SigningPKCS8 types.String `tfsdk:"pkcs8"`
	SigningKey   types.String `tfsdk:"signing_key"`
	Issuer       types.String `tfsdk:"issuer"`
	JWKSKeys     types.String `tfsdk:"jwks_keys"`
}

// Schema implements K3sType.
func (m OidcConfig) Schema() schema.Attribute {
	return schema.SingleNestedAttribute{
		Optional:    true,
		Description: "Support for including oidc provider in k3s",
		Attributes: map[string]schema.Attribute{
			"audience": schema.StringAttribute{
				Required:            true,
				Sensitive:           true,
				MarkdownDescription: "OIDC Audience",
			},
			"pkcs8": schema.StringAttribute{
				Required:            true,
				Sensitive:           true,
				MarkdownDescription: "Public signing key",
			},
			"signing_key": schema.StringAttribute{
				Required:            true,
				Sensitive:           true,
				MarkdownDescription: "Private signing key",
			},
			"issuer": schema.StringAttribute{
				Required:            true,
				Sensitive:           true,
				MarkdownDescription: "Issuer url",
			},
			"jwks_keys": schema.StringAttribute{
				Computed:            true,
				Sensitive:           true,
				MarkdownDescription: "JSON web key set generated by the cluster following API server configuration",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}

var _ K3sType = &OidcConfig{}

func (n OidcConfig) Validate() error {
	return nil
}

func (m *OidcConfig) ToObject(ctx context.Context) basetypes.ObjectValue {
	return ToObject(ctx, m)
}

func NewOidcConfig(ctx context.Context, t basetypes.ObjectValue) OidcConfig {
	var na OidcConfig
	t.As(ctx, &na, basetypes.ObjectAsOptions{})
	return na
}

func (m OidcConfig) AttributeTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"audience":    types.StringType,
		"pkcs8":       types.StringType,
		"signing_key": types.StringType,
		"issuer":      types.StringType,
		"jwks_keys":   types.StringType,
	}
}

type HaConfig struct {
	ClusterInit types.Bool   `tfsdk:"cluster_init"`
	Token       types.String `tfsdk:"token"`
	Server      types.String `tfsdk:"server"`
	KubeConfig  types.String `tfsdk:"kubeconfig"`
}

// Schema implements K3sType.
func (m HaConfig) Schema() schema.Attribute {
	return schema.SingleNestedAttribute{
		Optional:    true,
		Description: "Run server node in highly available mode",
		PlanModifiers: []planmodifier.Object{
			objectplanmodifier.UseStateForUnknown(),
		},
		Attributes: map[string]schema.Attribute{
			"cluster_init": schema.BoolAttribute{
				Computed:            true,
				Optional:            true,
				Default:             booldefault.StaticBool(false),
				MarkdownDescription: "Node is the init node for the HA cluster",
			},
			"server": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Url of init node",
			},
			"token": schema.StringAttribute{
				Optional:            true,
				Sensitive:           true,
				MarkdownDescription: "Server token used for joining nodes to the cluster",
			},
			"kubeconfig": schema.StringAttribute{
				Optional:            true,
				Sensitive:           true,
				MarkdownDescription: "KubeConfig for the cluster",
			},
		},
	}
}

func NewHaConfig(ctx context.Context, t basetypes.ObjectValue) HaConfig {
	var na HaConfig
	t.As(ctx, &na, basetypes.ObjectAsOptions{})
	return na
}

func (m *HaConfig) ToObject(ctx context.Context) basetypes.ObjectValue {
	return ToObject(ctx, m)
}

func (m HaConfig) AttributeTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"cluster_init": types.BoolType,
		"token":        types.StringType,
		"server":       types.StringType,
		"kubeconfig":   types.StringType,
	}
}

var _ K3sType = &HaConfig{}

func (h HaConfig) Validate() error {
	if !h.ClusterInit.ValueBool() && (h.Token.IsNull() || h.Server.IsNull()) {
		return fmt.Errorf("when not in cluster-init, token and server must be passed")
	}
	if h.ClusterInit.ValueBool() && (!h.Token.IsNull() || !h.Server.IsNull()) {
		return fmt.Errorf("when in cluster-init, token and server must not be passed")
	}
	return nil
}
