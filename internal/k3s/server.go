package k3s

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-log/tflog"
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
	registry   map[string]any
	token      string
	kubeConfig string
	version    *string
	ctx        context.Context
	binDir     string
}

// KubeConfig implements K3sServer.
func (s *server) KubeConfig() string {
	return s.kubeConfig
}

// Token implements K3sServer.
func (s *server) Token() string {
	return s.token
}

func NewK3sServerComponent(ctx context.Context, config map[string]any, registry map[string]any, version *string, binDir string) K3sServer {

	return &server{
		ctx:      ctx,
		config:   config,
		registry: registry,
		version:  version,
		binDir:   binDir,
	}
}

func (s *server) dataDir() string {
	if dir, ok := s.config["data_dir"].(string); ok && dir != "" {
		return dir
	}
	return DATA_DIR
}

// RunPreReqs implements K3sComponent.
func (s *server) RunPreReqs(client ssh_client.SSHClient) error {

	if err := client.WaitForReady(); err != nil {
		return err
	}

	tflog.Debug(s.ctx, "Reading config path")
	configPath := fmt.Sprintf("%s/config.yaml", CONFIG_DIR)
	configContents, err := yaml.Marshal(s.config)
	if err != nil {
		return err
	}

	tflog.Debug(s.ctx, "Reading registries")
	registryContents := []byte{}
	registryPath := fmt.Sprintf("%s/registries.yaml", CONFIG_DIR)
	if s.registry != nil {
		registryContents, err = yaml.Marshal(s.registry)
		if err != nil {
			return err
		}
	}

	systemDContent, err := ReadSystemDSingleServer(configPath, s.binDir)
	if err != nil {
		return err
	}

	tflog.Debug(s.ctx, "Reading install script")
	installContents, err := ReadInstallScript()
	if err != nil {
		return err
	}

	commands := []string{
		// Move over Install script
		fmt.Sprintf("echo %q | sudo tee %s/k3s-install.tmp.sh > /dev/null", installContents, s.binDir),
		fmt.Sprintf("sudo base64 -d %[1]s/k3s-install.tmp.sh | sudo tee %[1]s/k3s-install.sh > /dev/null", s.binDir),
		fmt.Sprintf("sudo rm %s/k3s-install.tmp.sh", s.binDir),
		// Ensure directories exist
		fmt.Sprintf("sudo mkdir -p %s", s.dataDir()),
		fmt.Sprintf("sudo mkdir -p %s", CONFIG_DIR),
		// Write config file
		fmt.Sprintf("echo %q | sudo tee %s.tmp > /dev/null", base64.StdEncoding.EncodeToString(configContents), CONFIG_DIR),
		fmt.Sprintf("sudo base64 -d %s.tmp | sudo tee %s > /dev/null", CONFIG_DIR, configPath),
		fmt.Sprintf("sudo rm %s.tmp", CONFIG_DIR),
		// Write the SystemD file
		fmt.Sprintf("echo %q | sudo tee /etc/systemd/system/k3s.service.tmp > /dev/null", systemDContent),
		"sudo base64 -d /etc/systemd/system/k3s.service.tmp | sudo tee /etc/systemd/system/k3s.service > /dev/null",
		"sudo chown root:root /etc/systemd/system/k3s.service",
		"sudo rm /etc/systemd/system/k3s.service.tmp",
	}

	if len(registryContents) != 0 {
		commands = append(commands, []string{
			// Write registries file
			fmt.Sprintf("echo %q | sudo tee %s.tmp > /dev/null", base64.StdEncoding.EncodeToString(registryContents), CONFIG_DIR),
			fmt.Sprintf("sudo base64 -d %s.tmp | sudo tee %s > /dev/null", CONFIG_DIR, registryPath),
			fmt.Sprintf("sudo rm %s.tmp", CONFIG_DIR),
		}...)
	}

	return client.RunStream(commands)
}

// RunInstall implements K3sComponent.
func (s *server) RunInstall(client ssh_client.SSHClient) error {
	flags := []string{
		"INSTALL_K3S_SKIP_START=true",
	}

	if s.version != nil {
		flags = append(flags, fmt.Sprintf("INSTALL_K3S_VERSION=\"%s\"", *s.version))
	}

	commands := []string{
		fmt.Sprintf("sudo BIN_DIR=%[1]s %s bash %[1]s/k3s-install.sh", s.binDir, strings.Join(flags, " ")),
		"sudo systemctl daemon-reload",
		"sudo systemctl start k3s",
	}

	if err := client.RunStream(commands); err != nil {
		return err
	}

	// If first node on HA, set token
	if s.token == "" {
		token, err := client.Run("sudo cat /var/lib/rancher/k3s/server/token")
		if err != nil {
			return err
		}
		if len(token) != 1 {
			return fmt.Errorf("mismatched return from grapping server token")
		}
		s.token = token[0]
	}

	// Retrieve kubeconfig
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
func (s *server) RunUninstall(client ssh_client.SSHClient) error {
	return client.RunStream([]string{
		fmt.Sprintf("sudo bash %s/k3s-uninstall.sh", s.binDir),
	})
}

func (s *server) Status(client ssh_client.SSHClient) (bool, error) {
	return systemdStatus("k3s", client)

}

func (s *server) Journal(client ssh_client.SSHClient) (string, error) {
	res, err := client.Run("sudo journalctl -xeu k3s")
	if err != nil {
		return "", err
	}

	if len(res) != 1 {
		return "", fmt.Errorf("wrong number of results from server status check")
	}

	return res[0], nil
}

func (s *server) StatusLog(client ssh_client.SSHClient) (string, error) {
	res, err := client.Run("sudo systemctl status k3s")
	if err != nil {
		return "", err
	}

	if len(res) != 1 {
		return "", fmt.Errorf("wrong number of results from server status check")
	}

	return res[0], nil
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
