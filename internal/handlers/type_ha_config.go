package handlers

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/objectplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"striveworks.us/terraform-provider-k3s/internal/k3s"
)

type HaConfig struct {
	ClusterInit types.Bool   `tfsdk:"cluster_init"`
	Token       types.String `tfsdk:"token"`
	Server      types.String `tfsdk:"server"`
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
		},
	}
}

func (h HaConfig) configureServer(server k3s.ServerHaMode) {
	server.AddHA(h.ClusterInit.ValueBool(), h.Token.ValueString(), h.Server.ValueString())
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
	}
}

func (h HaConfig) Validate() error {
	if !h.ClusterInit.ValueBool() && (h.Token.IsNull() || h.Server.IsNull()) {
		return fmt.Errorf("when not in cluster-init, token and server must be passed")
	}
	if h.ClusterInit.ValueBool() && (!h.Token.IsNull() || !h.Server.IsNull()) {
		return fmt.Errorf("when in cluster-init, token and server must not be passed")
	}
	return nil
}
