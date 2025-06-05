package k3s

import (
	"encoding/base64"
	"fmt"
	"strings"

	"gopkg.in/yaml.v2"
	"k8s.io/client-go/tools/clientcmd"
	"striveworks.us/terraform-provider-k3s/internal/ssh_client"
)

type K3sServer interface {
	K3sComponent
	Token() string
	KubeConfig() string
}

var _ K3sServer = &server{}

type server struct {
	config     map[string]any
	token      string
	kubeConfig string
	version    *string
}

// KubeConfig implements K3sServer.
func (s *server) KubeConfig() string {
	return s.kubeConfig
}

// Token implements K3sServer.
func (s *server) Token() string {
	return s.token
}

func NewK3sServerComponent(config map[string]any, version *string) K3sServer {
	return &server{config: config, version: version}
}

func (s *server) dataDir() string {
	if dir, ok := s.config["data_dir"].(string); ok && dir != "" {
		return dir
	}
	return DATA_DIR
}

// RunPreReqs implements K3sComponent.
func (s *server) RunPreReqs(client ssh_client.SSHClient, callbacks ...func(string)) error {

	client.WaitForReady(callbacks[0])

	configPath := fmt.Sprintf("%s/config.yaml", CONFIG_DIR)
	configContents, err := yaml.Marshal(s.config)
	if err != nil {
		return err
	}

	systemDContent, err := ReadSystemDSingle(configPath)
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
		fmt.Sprintf("sudo mkdir -p %s", s.dataDir()),
		fmt.Sprintf("sudo mkdir -p %s", CONFIG_DIR),
		// Write config file
		fmt.Sprintf("echo %q | sudo tee %s/config.tmp.yaml > /dev/null", base64.StdEncoding.EncodeToString(configContents), CONFIG_DIR),
		fmt.Sprintf("sudo base64 -d %s/config.tmp.yaml | sudo tee %s > /dev/null", CONFIG_DIR, configPath),
		fmt.Sprintf("sudo rm %s/config.tmp.yaml", CONFIG_DIR),
		// Write the SystemD file
		fmt.Sprintf("echo %q | sudo tee /etc/systemd/system/k3s.service.tmp > /dev/null", systemDContent),
		"sudo base64 -d /etc/systemd/system/k3s.service.tmp | sudo tee /etc/systemd/system/k3s.service > /dev/null",
		"sudo chown root:root /etc/systemd/system/k3s.service",
		"sudo rm /etc/systemd/system/k3s.service.tmp",
	}

	return client.RunStream(commands, callbacks...)
}

// RunInstall implements K3sComponent.
func (s *server) RunInstall(client ssh_client.SSHClient, callbacks ...func(string)) error {
	version := ""
	if s.version != nil {
		version = fmt.Sprintf("INSTALL_K3S_VERSION=%s", *s.version)
	}
	commands := []string{
		fmt.Sprintf("sudo INSTALL_K3S_SKIP_START=true %s bash /usr/local/bin/k3s-install.sh", version),
		"sudo systemctl daemon-reload",
		"sudo systemctl start k3s",
	}

	if err := client.RunStream(commands, callbacks...); err != nil {
		return err
	}

	// Gather Items that are needed to be shared between hosts
	token, err := client.Run("sudo cat /var/lib/rancher/k3s/server/token")
	if err != nil {
		return err
	}
	if len(token) != 1 {
		return fmt.Errorf("mismatched return from grapping server token")
	}
	s.token = token[0]

	kubeconfig, err := client.Run("sudo cat /etc/rancher/k3s/k3s.yaml")
	if err != nil {
		return err
	}
	if len(kubeconfig) != 1 {
		return fmt.Errorf("mismatched return from grapping server token")
	}
	kubeConfig, err := updateKubeConfig(kubeconfig[0], client.Host())
	if err != nil {
		return err
	}
	s.kubeConfig = kubeConfig

	return nil
}

// RunUninstall implements K3sServer.
func (s *server) RunUninstall(client ssh_client.SSHClient, callbacks ...func(string)) error {
	return client.RunStream([]string{
		"sudo bash /usr/local/bin/k3s-uninstall.sh",
	}, callbacks...)
}

func (s *server) Status(client ssh_client.SSHClient) (bool, error) {
	res, err := client.Run("sudo systemctl is-active k3s")
	if err != nil {
		return false, err
	}

	if len(res) != 1 {
		return false, fmt.Errorf("wrong number of results from server status check")
	}

	return (res[0] == "active"), nil

}

func updateKubeConfig(kubeconfigText string, host string) (string, error) {
	config, err := clientcmd.Load([]byte(kubeconfigText))
	if err != nil {
		return "", err
	}

	this := *config.Clusters["default"]
	this.Server = fmt.Sprintf("https://%s:6443", strings.ReplaceAll(host, ":22", ""))
	config.Clusters["default"] = &this

	fixed, err := clientcmd.Write(*config)
	if err != nil {
		return "", err
	}

	return string(fixed), nil
}
