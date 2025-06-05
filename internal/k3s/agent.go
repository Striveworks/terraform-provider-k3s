// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package k3s

import "striveworks.us/terraform-provider-k3s/internal/ssh_client"

type K3sAgent interface {
	K3sComponent
}

var _ K3sAgent = &agent{}

type agent struct {
	config  map[string]any
	token   string
	version *string
}

func NewK3sAgentComponent(config map[string]any, token string, version *string) K3sAgent {
	return &server{config: config, token: token, version: version}
}

// RunInstall implements K3sAgent.
func (a *agent) RunInstall(ssh_client.SSHClient, ...func(string)) error {
	panic("unimplemented")
}

// RunPreReqs implements K3sAgent.
func (a *agent) RunPreReqs(ssh_client.SSHClient, ...func(string)) error {
	panic("unimplemented")
}

// RunUninstall implements K3sAgent.
func (a *agent) RunUninstall(ssh_client.SSHClient, ...func(string)) error {
	panic("unimplemented")
}

// Status implements K3sAgent.
func (a *agent) Status(ssh_client.SSHClient) (bool, error) {
	panic("unimplemented")
}
