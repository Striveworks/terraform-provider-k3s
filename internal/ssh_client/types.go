package ssh_client

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	datasourceschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int32default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	tftypes "github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"striveworks.us/terraform-provider-k3s/internal/schemas"
)

var _ schemas.K3sTypeSchema = &SSHConfig{}

type SSHConfig struct {
	User           tftypes.String `tfsdk:"user"`
	Host           tftypes.String `tfsdk:"host"`
	Port           tftypes.Int32  `tfsdk:"port"`
	PrivateKey     tftypes.String `tfsdk:"private_key"`
	Password       tftypes.String `tfsdk:"password"`
	PrivateKeyFile tftypes.String `tfsdk:"private_key_file"`
	HostKey        tftypes.String `tfsdk:"host_key"`
	HostKeyFile    tftypes.String `tfsdk:"host_key_file"`
}

// AttributeTypes implements [schemas.K3sTypeSchema].
func (s SSHConfig) AttributeTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"user":             tftypes.StringType,
		"host":             tftypes.StringType,
		"port":             tftypes.Int32Type,
		"private_key":      tftypes.StringType,
		"private_key_file": tftypes.StringType,
		"password":         tftypes.StringType,
		"host_key":         tftypes.StringType,
		"host_key_file":    tftypes.StringType,
	}
}

// Schema implements [schemas.K3sTypeSchema].
func (s SSHConfig) Schema() schema.Attribute {
	return schema.SingleNestedAttribute{
		Required: true,
		Description: `SSH authentication config. At least one of password, private_key, or private_key_file must be provided.
		If multiple credential types are provided, each is added to the SSH auth methods.
		For host key verification, host_key or host_key_file can be passed in, otherwise host key verification is ignored.
		`,
		Attributes: map[string]schema.Attribute{
			"user": schema.StringAttribute{
				Required:            true,
				Sensitive:           false,
				MarkdownDescription: "SSH User",
			},
			"host": schema.StringAttribute{
				Required:            true,
				Sensitive:           false,
				MarkdownDescription: "Hostname or IP Address",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"port": schema.Int32Attribute{
				Optional:            true,
				Computed:            true,
				Sensitive:           false,
				MarkdownDescription: "SSH Port",
				Default:             int32default.StaticInt32(22),
			},
			"private_key": schema.StringAttribute{
				Optional:            true,
				Sensitive:           true,
				MarkdownDescription: "Inline private key in PEM format",
			},
			"private_key_file": schema.StringAttribute{
				Optional:            true,
				Sensitive:           true,
				MarkdownDescription: "Path to pem file",
			},
			"password": schema.StringAttribute{
				Optional:            true,
				Sensitive:           true,
				MarkdownDescription: "SSH Password",
			},
			"host_key": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Inline SSH host public key",
			},
			"host_key_file": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Path to SSH host public key",
			},
		},
	}
}

func (s SSHConfig) DataSourceSchema() datasourceschema.Attribute {
	return datasourceschema.SingleNestedAttribute{
		Required:    true,
		Description: "SSH authentication config. At least one of password, private_key, or private_key_file must be provided. If multiple credential types are provided, each is added to the SSH auth methods.",
		Attributes: map[string]datasourceschema.Attribute{
			"user": datasourceschema.StringAttribute{
				Required:            true,
				Sensitive:           false,
				MarkdownDescription: "SSH User",
			},
			"host": datasourceschema.StringAttribute{
				Required:            true,
				Sensitive:           false,
				MarkdownDescription: "Hostname or IP Address",
			},
			"port": datasourceschema.Int32Attribute{
				Optional:            true,
				Sensitive:           false,
				MarkdownDescription: "SSH Port. Defaults to 22 when omitted.",
			},
			"private_key": datasourceschema.StringAttribute{
				Optional:            true,
				Sensitive:           true,
				MarkdownDescription: "Inline private key in PEM format",
			},
			"private_key_file": datasourceschema.StringAttribute{
				Optional:            true,
				Sensitive:           true,
				MarkdownDescription: "Path to pem file",
			},
			"password": datasourceschema.StringAttribute{
				Optional:            true,
				Sensitive:           true,
				MarkdownDescription: "SSH Password",
			},
			"host_key": datasourceschema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Inline SSH host public key",
			},
			"host_key_file": datasourceschema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Path to SSH host public key",
			},
		},
	}
}

// ToObject implements [schemas.K3sTypeSchema].
func (s *SSHConfig) ToObject(ctx context.Context) basetypes.ObjectValue {
	return schemas.ToObject(ctx, s)
}

// Validate implements [schemas.K3sTypeSchema].
func (s *SSHConfig) Validate() error {
	if s.PrivateKey.IsUnknown() || s.PrivateKeyFile.IsUnknown() || s.Password.IsUnknown() {
		return nil
	}
	if s.PrivateKey.ValueString() == "" && s.PrivateKeyFile.ValueString() == "" && s.Password.ValueString() == "" {
		return fmt.Errorf("either password, private_key or private_key_file must be provided")
	}
	return nil
}
