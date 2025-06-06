// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

// Terraform markdown formatter, wraps string in markdown terraform blocks.
func TfMd(contents string) string {
	return "```terraform\n" + contents + "\n```"
}
