package handlers

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	tftypes "github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"striveworks.us/terraform-provider-k3s/internal/ssh_client"
)

type K3sTypeSchema interface {
	// The schema for the terraform block
	Schema() schema.Attribute
}

type K3sTypeAttr interface {
	// The attributes to convert to and from `basetypes.ObjectValue`
	AttributeTypes() map[string]attr.Type
}

type K3sTypeToObject interface {
	// Converts back to ObjectValue
	ToObject(context.Context) basetypes.ObjectValue
}

type K3sTypeValidate interface {
	// Any validation to be run on the type
	Validate() error
}

type K3sTypeSSH interface {
	// Type can create a SSHClient
	SshClient(ctx context.Context) (ssh_client.SSHClient, error)
}

func ToObject[T K3sTypeAttr](ctx context.Context, o T) basetypes.ObjectValue {
	obj, _ := tftypes.ObjectValueFrom(ctx, o.AttributeTypes(), o)
	return obj
}
