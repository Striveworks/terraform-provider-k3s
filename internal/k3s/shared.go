package k3s

import (
	"context"
	"encoding/base64"
	"fmt"
	"regexp"

	"github.com/hashicorp/terraform-plugin-log/tflog"
	"gopkg.in/yaml.v2"
	"striveworks.us/terraform-provider-k3s/internal/ssh_client"
)

const DATA_DIR string = "/var/lib/rancher/k3s"
const CONFIG_DIR string = "/etc/rancher/k3s"

type K3sComponent interface {
	// Ensures all files and configs are present on remote node.
	RunPreReqs(ssh_client.SSHClient) error
	// Runs the install script, should be ran after `RunPreReqs`.
	RunInstall(ssh_client.SSHClient) error
	// Runs the k3s uninstall script that is included.
	// with the install
	RunUninstall(ssh_client.SSHClient) error
	// Runs an update operation on the k3s node. If
	// it's a simple config change, this will result
	// in a systemd restart
	Update(ssh_client.SSHClient) error
	// Queries if the systemd service is active.
	Status(ssh_client.SSHClient) (bool, error)
	// Gets systemd status.
	StatusLog(ssh_client.SSHClient) (string, error)
	// Gets the journalctl logs.
	Journal(ssh_client.SSHClient) (string, error)
	// Resyncs node object with remote.
	Resync(ssh_client.SSHClient) error
	// The server token used to join the cluster
	// Running `Resync` first will ensure this is set.
	Token() string
}

func systemdStatus(unit string, client ssh_client.SSHClient) (bool, error) {
	res, err := client.Run(fmt.Sprintf("sudo systemctl is-active %s", unit))
	if err != nil {
		return false, err
	}

	if len(res) != 1 {
		return false, fmt.Errorf("wrong number of results from server status check")
	}

	active := regexp.MustCompile(`\s+`).ReplaceAllString(res[0], "")
	return (active == "active"), nil
}

// Commands for configuring server/agent config.
func configCommands(ctx context.Context, config map[any]any) ([]string, error) {
	tflog.Debug(ctx, "Reading config path")
	configPath := fmt.Sprintf("%s/config.yaml", CONFIG_DIR)
	configContents, err := yaml.Marshal(config)
	if err != nil {
		return []string{}, err
	}

	return []string{
		// Write config file
		fmt.Sprintf("echo %q | sudo tee %s.tmp > /dev/null", base64.StdEncoding.EncodeToString(configContents), CONFIG_DIR),
		fmt.Sprintf("sudo base64 -d %s.tmp | sudo tee %s > /dev/null", CONFIG_DIR, configPath),
		fmt.Sprintf("sudo rm %s.tmp", CONFIG_DIR),
	}, nil
}

// Commands for configuring server/agent registry.
func registryCommands(ctx context.Context, registry map[any]any) (commands []string, err error) {
	tflog.Debug(ctx, "Reading registries")

	registryPath := fmt.Sprintf("%s/registries.yaml", CONFIG_DIR)
	var registryContents []byte
	if registry != nil {
		registryContents, err = yaml.Marshal(registry)
		if err != nil {
			return []string{}, err
		}
	}

	if len(registryContents) != 0 {
		commands = []string{
			// Write registries file
			fmt.Sprintf("echo %q | sudo tee %s.tmp > /dev/null", base64.StdEncoding.EncodeToString(registryContents), CONFIG_DIR),
			fmt.Sprintf("sudo base64 -d %s.tmp | sudo tee %s > /dev/null", CONFIG_DIR, registryPath),
			fmt.Sprintf("sudo rm %s.tmp", CONFIG_DIR),
		}
	}

	return commands, err
}

// Will import a remote yaml file.
func readYaml(client ssh_client.SSHClient, file string, missingOk ...bool) (map[any]any, error) {
	res, err := client.ReadFile(file, len(missingOk) > 0 && missingOk[0], true)
	if err != nil {
		return nil, err
	}

	if res == "" {
		return nil, nil
	}

	var contents map[any]any
	if err := yaml.Unmarshal([]byte(res), &contents); err != nil {
		return nil, err
	}
	return contents, nil
}

func getConfig(client ssh_client.SSHClient) (map[any]any, error) {
	return readYaml(client, "/etc/rancher/k3s/config.yaml", true)
}

// Retrieve kubeconfig.
func getRegistry(client ssh_client.SSHClient) (map[any]any, error) {
	return readYaml(client, "/etc/rancher/k3s/registry.yaml", true)
}
