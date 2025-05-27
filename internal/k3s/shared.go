// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package k3s

import (
	"striveworks.us/terraform-provider-k3s/internal/ssh_client"
)

const DATA_DIR string = "/var/lib/rancher/k3s"
const CONFIG_DIR string = "/etc/rancher/k3s"

type K3sComponent interface {
	RunPreReqs(ssh_client.SSHClient, ...func(string)) error
	RunInstall(ssh_client.SSHClient, ...func(string)) error
	RunUninstall(ssh_client.SSHClient, ...func(string)) error
	Status(ssh_client.SSHClient) (bool, error)
}
