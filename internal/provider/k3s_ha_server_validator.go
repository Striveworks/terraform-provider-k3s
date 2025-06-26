package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/resource"
)

type k3sHaServerAuthValidator struct{}

var _ resource.ConfigValidator = &k3sHaServerAuthValidator{}

func NewK3sHaServerValidator() resource.ConfigValidator {
	return &k3sHaServerAuthValidator{}
}

// Description implements resource.ConfigValidator.
func (k *k3sHaServerAuthValidator) Description(context.Context) string {
	return "Validates for high available server"
}

// MarkdownDescription implements resource.ConfigValidator.
func (k *k3sHaServerAuthValidator) MarkdownDescription(context.Context) string {
	var desc MarkdownDescription = `
Allows either Password or Private Key, but not both
`

	return desc.ToMarkdown()
}

// ValidateResource implements resource.ConfigValidator.
func (k *k3sHaServerAuthValidator) ValidateResource(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var data HaServerClientModel

	// Read Terraform state data into the model
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Read node block data into the model
	nodes := make([]HaServerNodeModel, 0, len(data.Nodes.Elements()))
	resp.Diagnostics.Append(data.Nodes.ElementsAs(ctx, &nodes, false)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if len(nodes) < 3 || len(nodes)%2 == 0 {
		resp.Diagnostics.AddError("Wrong node count", "K3s High Availability must be >=3 and odd in count")
		return
	}

	globalAuth := !data.PrivateKey.IsNull() || !data.Password.IsNull()
	for _, elm := range nodes {
		if !globalAuth && elm.PrivateKey.IsNull() && elm.Password.IsNull() {
			resp.Diagnostics.AddError("Missing auth", fmt.Sprintf("No auth configured for host %v", elm.Host.ValueString()))
		}
	}

	if resp.Diagnostics.HasError() {
		return
	}

}
