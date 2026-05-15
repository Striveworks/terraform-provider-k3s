package schemas

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	tftypes "github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
)

// K3sTypeSchema defines a contract for types that represent K3s resource
// schemas within the Terraform provider. Implementations of this interface
// provide the necessary methods to define the Terraform schema, manage
// attribute type conversions, facilitate object serialization, and
// encapsulate validation logic specific to the K3s resource.
type K3sTypeSchema interface {
	// Schema returns the Terraform schema definition for the implementing type.
	// This schema is used by the Terraform plugin framework to understand the
	// structure and types of the resource or data source being managed.
	Schema() schema.Attribute
	// AttributeTypes returns a map of attribute names to their corresponding
	// Terraform attribute types (`attr.Type`). This map is crucial for the
	// Terraform plugin framework to correctly handle type conversions when
	// working with `basetypes.ObjectValue`.
	AttributeTypes() map[string]attr.Type
	// ToObject converts the implementing type instance into a `basetypes.ObjectValue`.
	// This is typically used to serialize the Go struct into a format compatible
	// with Terraform's internal value representation for state management and
	// plan generation.
	ToObject(context.Context) basetypes.ObjectValue
	// Validate performs any custom validation checks on the implementing type's
	// current state. This method allows for complex validation logic that
	// cannot be expressed solely through schema-level validation rules.
	Validate() error
}

// ToObject is a generic helper function that converts a K3sTypeSchema
// implementation into a `basetypes.ObjectValue`. It leverages the
// `AttributeTypes` method of the K3sTypeSchema to correctly map
// the Go struct fields to their corresponding Terraform types, facilitating
// the creation of Terraform-compatible objects for state and plan management.
func ToObject[T K3sTypeSchema](ctx context.Context, o T) basetypes.ObjectValue {
	obj, _ := tftypes.ObjectValueFrom(ctx, o.AttributeTypes(), o)
	return obj
}
