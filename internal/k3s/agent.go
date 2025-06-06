// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package k3s

import (
	"encoding/base64"
	"fmt"

	"gopkg.in/yaml.v2"
	"striveworks.us/terraform-provider-k3s/internal/ssh_client"
)

type K3sAgent interface {
	K3sComponent
}

var _ K3sAgent = &agent{}

type agent struct {
	config  map[string]any
	version *string
}

func NewK3sAgentComponent(config map[string]any, version *string) K3sAgent {
	return &server{config: config, version: version}
}

// RunInstall implements K3sAgent.
func (a *agent) RunInstall(client ssh_client.SSHClient, callbacks ...func(string)) error {
	version := ""
	if a.version != nil {
		version = fmt.Sprintf("INSTALL_K3S_VERSION=%s", *a.version)
	}

	commands := []string{
		fmt.Sprintf("sudo INSTALL_K3S_SKIP_START=true %s bash /usr/local/bin/k3s-install.sh", version),
		"sudo systemctl daemon-reload",
		"sudo systemctl start k3s",
	}

	if err := client.RunStream(commands, callbacks...); err != nil {
		return err
	}
	return nil
}

// RunPreReqs implements K3sAgent.
func (a *agent) RunPreReqs(client ssh_client.SSHClient, callbacks ...func(string)) error {

	if err := client.WaitForReady(callbacks[0]); err != nil {
		return err
	}

	configPath := fmt.Sprintf("%s/config.yaml", CONFIG_DIR)
	configContents, err := yaml.Marshal(a.config)
	if err != nil {
		return err
	}

	systemDContent, err := ReadSystemDSingleAgent(configPath)
	if err != nil {
		return err
	}

	installContents, err := ReadInstallScript()
	if err != nil {
		return err
	}

	commands := []string{
		// Move over Install script
		fmt.Sprintf("echo %q | sudo tee /usr/local/bin/k3s-install.tmp.sh > /dev/null", installContents),
		"sudo base64 -d /usr/local/bin/k3s-install.tmp.sh | sudo tee /usr/local/bin/k3s-install.sh > /dev/null",
		"sudo rm /usr/local/bin/k3s-install.tmp.sh",
		// Ensure directories exist
		fmt.Sprintf("sudo mkdir -p %s", a.dataDir()),
		fmt.Sprintf("sudo mkdir -p %s", CONFIG_DIR),
		// Write config file
		fmt.Sprintf("echo %q | sudo tee %s/config.tmp.yaml > /dev/null", base64.StdEncoding.EncodeToString(configContents), CONFIG_DIR),
		fmt.Sprintf("sudo base64 -d %s/config.tmp.yaml | sudo tee %s > /dev/null", CONFIG_DIR, configPath),
		fmt.Sprintf("sudo rm %s/config.tmp.yaml", CONFIG_DIR),
		// Write the SystemD file
		fmt.Sprintf("echo %q | sudo tee /etc/systemd/system/k3s-agent.service.tmp > /dev/null", systemDContent),
		"sudo base64 -d /etc/systemd/system/k3s-agent.service.tmp | sudo tee /etc/systemd/system/k3s-agent.service > /dev/null",
		"sudo chown root:root /etc/systemd/system/k3s-agent.service",
		"sudo rm /etc/systemd/system/k3s-agent.service.tmp",
	}

	return client.RunStream(commands, callbacks...)

}

// RunUninstall implements K3sAgent.
func (a *agent) RunUninstall(client ssh_client.SSHClient, callbacks ...func(string)) error {
	return client.RunStream([]string{
		"sudo bash /usr/local/bin/k3s-agent-uninstall.sh",
	}, callbacks...)
}

// Status implements K3sAgent.
func (a *agent) Status(client ssh_client.SSHClient) (bool, error) {
	res, err := client.Run("sudo systemctl is-active k3s")
	if err != nil {
		return false, err
	}

	if len(res) != 1 {
		return false, fmt.Errorf("wrong number of results from server status check")
	}

	return (res[0] == "active"), nil
}

func (a *agent) dataDir() string {
	if dir, ok := a.config["data_dir"].(string); ok && dir != "" {
		return dir
	}
	return DATA_DIR
}
