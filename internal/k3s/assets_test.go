// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package k3s

import (
	"testing"
)

func TestReadSystemDSingle(t *testing.T) {

	res, err := ReadSystemDSingleServer("/var/lib/rancher")
	if err != nil {
		t.Errorf("Error building systemd file: %s", err)
	}

	if len(res) < 1 {
		t.Errorf("Error building systemd file: %s", "Empty file")
	}
}
