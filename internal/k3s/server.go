package k3s

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/joho/godotenv"
	"k8s.io/client-go/tools/clientcmd"

	"striveworks.us/terraform-provider-k3s/internal/ssh_client"
)

type K3sServer interface {
	K3sComponent
	// Kubeconfig used for client access to the cluster
	KubeConfig() string
	// The k3s config
	Config() map[any]any
	// The k3s registry config
	Registry() map[any]any
	// Adds extra files to write to the node
	AddFile(path string, content string)
	// Get JWKS file
	JWKS(ssh_client.SSHClient) (string, error)
}

var _ K3sServer = &server{}

type server struct {
	config     map[any]any
	registry   map[any]any
	token      string
	kubeConfig string
	version    *string
	ctx        context.Context
	binDir     string
	// A map of target filepath : content
	extraFiles map[string]string
}

// JWKS implements K3sServer.
func (s *server) JWKS(client ssh_client.SSHClient) (string, error) {
	// use sudo just in case kubeconfig doesn't have wide permissions
	res, err := client.Run("sudo k3s kubectl get --raw /openid/v1/jwks")

	if err != nil {
		return "", err
	}
	return res[0], nil
}

// KubeConfig implements K3sServer.
func (s *server) KubeConfig() string {
	return s.kubeConfig
}

// Token implements K3sServer.
func (s *server) Token() string {
	return s.token
}

// Token implements K3sServer.
func (s *server) Config() map[any]any {
	return s.config
}

// Token implements K3sServer.
func (s *server) Registry() map[any]any {
	return s.registry
}

// New k3s server component.
func NewK3sServerComponent(ctx context.Context, config map[any]any, registry map[any]any, version *string, binDir string) K3sServer {
	return &server{
		ctx:        ctx,
		config:     config,
		registry:   registry,
		version:    version,
		binDir:     binDir,
		extraFiles: make(map[string]string),
	}
}

// New k3s ha server component meant to join a server that has already been initialized.
func NewK3sServerHAComponent(ctx context.Context, config map[any]any, registry map[any]any, version *string, token string, binDir string) K3sServer {
	return &server{
		ctx:        ctx,
		config:     config,
		registry:   registry,
		version:    version,
		binDir:     binDir,
		token:      token,
		extraFiles: make(map[string]string),
	}
}

func (s *server) AddFile(path string, content string) {
	s.extraFiles[path] = content
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

	cfgCommands, err := configCommands(s.ctx, s.config)
	if err != nil {
		return err
	}
	regCommands, err := registryCommands(s.ctx, s.registry)
	if err != nil {
		return err
	}

	extraFileCommands := s.syncExtraFiles()

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
	}
	// Write config file
	commands = append(commands, cfgCommands...)

	if len(regCommands) > 0 {
		commands = append(commands, regCommands...)
	}
	if len(extraFileCommands) > 0 {
		commands = append(commands, extraFileCommands...)
	}

	return client.RunStream(commands)
}

// RunInstall implements K3sComponent.
func (s *server) RunInstall(client ssh_client.SSHClient) (err error) {
	flags := []string{
		"INSTALL_K3S_SKIP_START=true",
		fmt.Sprintf("BIN_DIR=%s", s.binDir),
		fmt.Sprintf("INSTALL_K3S_EXEC='--config %s/config.yaml'", CONFIG_DIR),
	}

	// Join existing cluster as HA node
	if s.token != "" {
		flags = append(flags, fmt.Sprintf("K3S_TOKEN=%s", s.token))
	}

	if s.version != nil {
		flags = append(flags, fmt.Sprintf("INSTALL_K3S_VERSION=\"%s\"", *s.version))
	}

	commands := []string{
		fmt.Sprintf("sudo %s bash %s/k3s-install.sh", strings.Join(flags, " "), s.binDir),
		"sudo systemctl daemon-reload",
		"sudo systemctl start k3s",
	}

	if err = client.RunStream(commands); err != nil {
		return
	}

	// If first node on HA, set token
	if s.token == "" {
		s.token, err = s.getToken(client)
		if err != nil {
			return
		}
	}

	// Retrieve kubeconfig
	s.kubeConfig, err = s.getKubeConfig(client)

	return err
}

// RunUninstall implements K3sServer uninstall.
func (s *server) RunUninstall(client ssh_client.SSHClient, kubeconfig string) error {

	if err := deleteNode(s.ctx, kubeconfig, client.HostName()); err != nil {
		return err
	}
	return client.RunStream([]string{fmt.Sprintf("sudo bash %s/k3s-uninstall.sh", s.binDir)})
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

func (s *server) Update(client ssh_client.SSHClient) error {
	if err := client.WaitForReady(); err != nil {
		return err
	}

	commands, err := configCommands(s.ctx, s.config)
	if err != nil {
		return err
	}
	regCommands, err := registryCommands(s.ctx, s.registry)
	if err != nil {
		return err
	}

	if len(regCommands) != 0 {
		commands = append(commands, regCommands...)
	}

	return client.RunStream(append(commands, "sudo systemctl restart k3s"))
}

func (s *server) Resync(client ssh_client.SSHClient) (err error) {

	if s.token == "" {
		s.token, err = s.getToken(client)
		if err != nil {
			return
		}
	}

	s.kubeConfig, err = s.getKubeConfig(client)
	if err != nil {
		return
	}

	s.registry, err = getRegistry(client)
	if err != nil {
		return
	}

	s.config, err = getConfig(client)

	return
}

// Retrieve server token.
func (s *server) getToken(client ssh_client.SSHClient) (string, error) {
	// Look in default location
	token, err := client.ReadFile("/var/lib/rancher/k3s/server/token", true, true)
	if err != nil {
		return "", err
	}

	// Look in env file
	if token == "" {
		env, err := s.getServerEnv(client)
		if err != nil {
			return "", err
		}
		token = env["K3S_TOKEN"]
	}

	token = strings.Trim(token, "\n")
	tflog.MaskMessageStrings(s.ctx, token)
	return token, nil
}

// Retrieve kubeconfig.
func (s *server) getKubeConfig(client ssh_client.SSHClient) (string, error) {
	kubeconfig, err := client.ReadFile("/etc/rancher/k3s/k3s.yaml", false, true)
	if err != nil {
		return "", fmt.Errorf("could not retrieve kubeconfig: %s", err.Error())
	}

	kubeConfig, err := updateKubeConfig(kubeconfig, client.Host())
	if err != nil {
		return "", fmt.Errorf("could not retrieve server kubeconfig: %s", err.Error())
	}
	tflog.MaskMessageStrings(s.ctx, kubeConfig)
	return kubeConfig, nil
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

// Retrieve server token.
func (s *server) getServerEnv(client ssh_client.SSHClient) (map[string]string, error) {
	file, err := client.ReadFile("/etc/systemd/system/k3s.service.env", false, true)
	if err != nil {
		return nil, err
	}
	tflog.Info(s.ctx, fmt.Sprintf("K3s service file %v", file))
	return godotenv.Unmarshal(file)
}

func (s *server) syncExtraFiles() (commands []string) {
	for k, v := range s.extraFiles {
		commands = append(commands,
			fmt.Sprintf("sudo mkdir -p $(sudo realpath $(dirname %s))", k),
		)
		commands = append(commands, WriteFileCommands(k, base64.StdEncoding.EncodeToString([]byte(v)))...)
	}

	return
}
