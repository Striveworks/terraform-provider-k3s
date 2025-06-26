package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/resource"
)

type k3sServerAuthValdiator struct{}

var _ resource.ConfigValidator = &k3sServerAuthValdiator{}

func NewK3sServerValidator() resource.ConfigValidator {
	return &k3sServerAuthValdiator{}
}

// Description implements resource.ConfigValidator.
func (k *k3sServerAuthValdiator) Description(context.Context) string {
	return "Validates the authentication for the server"
}

// MarkdownDescription implements resource.ConfigValidator.
func (k *k3sServerAuthValdiator) MarkdownDescription(context.Context) string {
	var desc MarkdownDescription = `
Allows either Password or Private Key, but not both
`

	return desc.ToMarkdown()
}

// ValidateResource implements resource.ConfigValidator.
func (k *k3sServerAuthValdiator) ValidateResource(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var data ServerClientModel

	// Read Terraform state data into the model
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)

	if data.PrivateKey.IsNull() && data.Password.IsNull() {
		resp.Diagnostics.AddError("No auth", "Neither password nor private key was passed")
		return
	}

	if !data.PrivateKey.IsNull() && !data.Password.IsNull() {
		resp.Diagnostics.AddError("Conflicting auth", "Both password and private key were passed, only pass one")
		return
	}
}
