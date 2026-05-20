package k3s

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-log/tflog"
	"go.yaml.in/yaml/v2"
	"striveworks.us/terraform-provider-k3s/internal/ssh_client"
)

const DATA_DIR string = "/var/lib/rancher/k3s"
const CONFIG_DIR string = "/etc/rancher/k3s"
const BIN_DIR string = "/usr/local/bin"

type K3sComponent interface {
	Validate(context.Context) error
	PreInstall(context.Context, ssh_client.SSHClient) error
	Install(context.Context, ssh_client.SSHClient) error
	Uninstall(context.Context, ssh_client.SSHClient) error
	Refresh(context.Context, ssh_client.SSHClient) (bool, bool, error)
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

func k3sSystemdServiceExists(client ssh_client.SSHClient, serviceName string) (bool, error) {
	res, err := client.Run(fmt.Sprintf("sudo test -f /etc/systemd/system/%s.service && echo present || echo missing", serviceName))
	if err != nil {
		return false, err
	}
	if len(res) != 1 {
		return false, fmt.Errorf("wrong number of results from %s service existence check", serviceName)
	}

	return strings.TrimSpace(res[0]) == "present", nil
}

func k3sSystemdServiceActive(client ssh_client.SSHClient, serviceName string) (bool, error) {
	res, err := client.Run(fmt.Sprintf("sudo systemctl is-active --quiet %s && echo active || echo inactive", serviceName))
	if err != nil {
		return false, err
	}
	if len(res) != 1 {
		return false, fmt.Errorf("wrong number of results from %s service status check", serviceName)
	}

	return strings.TrimSpace(res[0]) == "active", nil
}

func restartFailedK3sSystemdService(client ssh_client.SSHClient, serviceName string) error {
	res, err := client.Run(fmt.Sprintf("sudo systemctl is-failed --quiet %[1]s && echo failed || echo not-failed", serviceName))
	if err != nil {
		return err
	}
	if len(res) != 1 {
		return fmt.Errorf("wrong number of results from %s service failed-state check", serviceName)
	}
	if strings.TrimSpace(res[0]) != "failed" {
		return nil
	}

	_, err = client.Run(fmt.Sprintf("sudo systemctl reset-failed %[1]s && sudo systemctl --no-block start %[1]s", serviceName))
	return err
}

func waitForK3sSystemdServiceActive(client ssh_client.SSHClient, serviceName string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	var lastErr error

	for time.Now().Before(deadline) {
		active, err := k3sSystemdServiceActive(client, serviceName)
		if err != nil {
			lastErr = err
		} else if active {
			return nil
		}

		if err := restartFailedK3sSystemdService(client, serviceName); err != nil {
			lastErr = err
		}

		time.Sleep(5 * time.Second)
	}

	journal, err := client.Run(fmt.Sprintf("sudo journalctl -u %s --no-pager -n 120 || true", serviceName))
	if err == nil && len(journal) == 1 && strings.TrimSpace(journal[0]) != "" {
		return fmt.Errorf("%s service did not become active within %s; recent journal:\n%s", serviceName, timeout, journal[0])
	}
	if lastErr != nil {
		return fmt.Errorf("%s service did not become active within %s: %w", serviceName, timeout, lastErr)
	}

	return fmt.Errorf("%s service did not become active within %s", serviceName, timeout)
}

func k3sBinaryVersion(client ssh_client.SSHClient, binDir string) (string, error) {
	if binDir == "" {
		binDir = BIN_DIR
	}

	res, err := client.Run(fmt.Sprintf("sudo %s/k3s -v", binDir))
	if err != nil {
		return "", err
	}
	if len(res) != 1 {
		return "", fmt.Errorf("wrong number of results from k3s version check")
	}

	version, err := parseK3sVersionOutput(res[0])
	if err != nil {
		return "", err
	}

	return version, nil
}

func parseK3sVersionOutput(output string) (string, error) {
	for _, line := range strings.Split(output, "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 3 && fields[0] == "k3s" && fields[1] == "version" {
			return fields[2], nil
		}
	}

	return "", fmt.Errorf("could not parse k3s version from output")
}
