package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/resource"
)

type k3sAgentAuthValdiator struct{}

var _ resource.ConfigValidator = &k3sAgentAuthValdiator{}

// Description implements resource.ConfigValidator.
func (k *k3sAgentAuthValdiator) Description(context.Context) string {
	return "Validates the authentication for the agent"
}

// MarkdownDescription implements resource.ConfigValidator.
func (k *k3sAgentAuthValdiator) MarkdownDescription(context.Context) string {
	var desc MarkdownDescription = `
Allows either Password or Private Key, but not both
`

	return desc.ToMarkdown()
}

// ValidateResource implements resource.ConfigValidator.
func (k *k3sAgentAuthValdiator) ValidateResource(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var data AgentClientModel

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
