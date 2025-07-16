package handlers

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int32default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/objectplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	tftypes "github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"striveworks.us/terraform-provider-k3s/internal/ssh_client"
)

type NodeAuth struct {
	Host       tftypes.String `tfsdk:"host"`
	Port       tftypes.Int32  `tfsdk:"port"`
	PrivateKey tftypes.String `tfsdk:"private_key"`
	Password   tftypes.String `tfsdk:"password"`
	User       tftypes.String `tfsdk:"user"`
}

func DefaultNodeAuth() basetypes.ObjectValue {
	return tftypes.ObjectNull(NodeAuth{}.AttributeTypes())
}

func NewNodeAuth(ctx context.Context, t basetypes.ObjectValue) NodeAuth {
	var na NodeAuth
	t.As(ctx, &na, basetypes.ObjectAsOptions{})
	tflog.Trace(ctx, "created node auth from terraform object type")
	return na
}

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
	return ssh_client.NewSSHClient(
		ctx, auth.Host.ValueString(),
		port,
		auth.User.ValueString(),
		auth.PrivateKey.ValueString(),
		auth.Password.ValueString(),
	)

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

func (NodeAuth) AttributeTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"host":        tftypes.StringType,
		"port":        tftypes.Int32Type,
		"private_key": tftypes.StringType,
		"password":    tftypes.StringType,
		"user":        tftypes.StringType,
	}
}
