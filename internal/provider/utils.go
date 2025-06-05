// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

// Terraform Markdown Formatting
func TfMd(contents string) string {
	return "```terraform\n" + contents + "\n```"
}
