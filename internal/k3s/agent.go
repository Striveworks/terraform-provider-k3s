// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package k3s

import (
	"context"
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
	ctx     context.Context
	binDir  string
}

func NewK3sAgentComponent(ctx context.Context, config map[string]any, version *string, binDir string) K3sAgent {
	return &agent{ctx: ctx, config: config, version: version, binDir: binDir}
}

// RunInstall implements K3sAgent.
func (a *agent) RunInstall(client ssh_client.SSHClient) error {
	version := ""
	if a.version != nil {
		version = fmt.Sprintf("INSTALL_K3S_VERSION='%s'", *a.version)
	}

	commands := []string{
		fmt.Sprintf("sudo BIN_DIR=%[1]s INSTALL_K3S_SKIP_START=true INSTALL_K3S_EXEC=agent %s bash %[1]s/k3s-install.sh", a.binDir, version),
		"sudo systemctl daemon-reload",
		"sudo systemctl start k3s-agent",
	}

	if err := client.RunStream(commands); err != nil {
		return err
	}
	return nil
}

// RunPreReqs implements K3sAgent.
func (a *agent) RunPreReqs(client ssh_client.SSHClient) error {

	if err := client.WaitForReady(); err != nil {
		return err
	}

	configPath := fmt.Sprintf("%s/config.yaml", CONFIG_DIR)
	configContents, err := yaml.Marshal(a.config)
	if err != nil {
		return err
	}

	systemDContent, err := ReadSystemDSingleAgent(configPath, a.binDir)
	if err != nil {
		return err
	}

	installContents, err := ReadInstallScript()
	if err != nil {
		return err
	}

	commands := []string{
		// Move over Install script
		fmt.Sprintf("echo %q | sudo tee %s/k3s-install.tmp.sh > /dev/null", installContents, a.binDir),
		fmt.Sprintf("sudo base64 -d %[1]s/k3s-install.tmp.sh | sudo tee %[1]s/k3s-install.sh > /dev/null", a.binDir),
		fmt.Sprintf("sudo rm %s/k3s-install.tmp.sh", a.binDir),
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

	return client.RunStream(commands)

}

// RunUninstall implements K3sAgent.
func (a *agent) RunUninstall(client ssh_client.SSHClient) error {
	return client.RunStream([]string{
		fmt.Sprintf("sudo bash %s/k3s-agent-uninstall.sh", a.binDir),
	})
}

func (a *agent) Journal(client ssh_client.SSHClient) (string, error) {
	res, err := client.Run("sudo journalctl -xeu k3s-agent")
	if err != nil {
		return "", err
	}

	if len(res) != 1 {
		return "", fmt.Errorf("wrong number of results from server status check")
	}

	return res[0], nil
}

// Status implements K3sAgent.
func (a *agent) Status(client ssh_client.SSHClient) (bool, error) {
	return systemdStatus("k3s-agent", client)
}

func (a *agent) StatusLog(client ssh_client.SSHClient) (string, error) {
	res, err := client.Run("sudo systemctl status k3s-agent")
	if err != nil {
		return "", err
	}

	if len(res) != 1 {
		return "", fmt.Errorf("wrong number of results from server status check")
	}

	return res[0], nil
}

func (a *agent) dataDir() string {
	if dir, ok := a.config["data_dir"].(string); ok && dir != "" {
		return dir
	}
	return DATA_DIR
}
