package k3s

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/joho/godotenv"
	"gopkg.in/yaml.v2"
	"k8s.io/client-go/tools/clientcmd"

	"striveworks.us/terraform-provider-k3s/internal/ssh_client"
)

type ServerKubeconfig interface {
	// Kubeconfig used for client access to the cluster
	KubeConfig() string
}

type ServerOidc interface {
	// Adds OIDC configuration
	AddOidc(audience string, issuer string, private_key string, signing_key string)
}

type ServerConfig interface {
	// The k3s config
	Config() string
}

type ServerHaMode interface {
	// The k3s config
	AddHA(cluster_init bool, token string, server string)
}

type Server interface {
	Component
	ServerKubeconfig
	ServerOidc
	ServerConfig
	ServerHaMode
}

var _ Server = &server{}

type server struct {
	config     map[any]any
	registry   map[any]any
	token      string
	kubeConfig string
	version    string
	ctx        context.Context
	binDir     string
	// A map of target filepath : content
	extraFiles map[string]string
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
func (s *server) Config() string {
	config, _ := yaml.Marshal(&s.config)
	return string(config)
}

func (s *server) AddHA(cluster_init bool, token string, server string) {
	if cluster_init {
		s.config["cluster-init"] = true
	}
	if token != "" {
		s.token = token
	}
	if server != "" {
		s.config["server"] = server
	}
}

// Easy constructor for using just uninstall and resync.
func NewK3ServerUninstall(ctx context.Context, binDir string) Server {
	return &server{ctx: ctx, binDir: binDir}
}

// New k3s ha server component meant to join a server that has already been initialized.
func NewK3sServerComponent(ctx context.Context, config string, registry string, version string, binDir string) (Server, error) {
	cfg := make(map[any]any)
	if err := yaml.Unmarshal([]byte(config), &cfg); err != nil {
		return nil, fmt.Errorf("parsing config: %s", err.Error())
	}

	reg := make(map[any]any)
	if err := yaml.Unmarshal([]byte(config), &reg); err != nil {
		return nil, fmt.Errorf("parsing registry: %s", err.Error())
	}

	return &server{
		ctx:        ctx,
		config:     cfg,
		registry:   reg,
		version:    version,
		binDir:     binDir,
		extraFiles: make(map[string]string),
	}, nil
}

func (s *server) AddOidc(audience string, issuer string, pkcs8_key string, signing_key string) {
	kube_api_server_args := []string{
		fmt.Sprintf("api-audiences=%s", audience),
		"service-account-key-file=/etc/rancher/k3s/tls/sa-signer-pkcs8.pub",
		"service-account-key-file=/var/lib/rancher/k3s/server/tls/service.key",
		"service-account-signing-key-file=/etc/rancher/k3s/tls/sa-signer.key",
		fmt.Sprintf("service-account-issuer=%s", issuer),
		"service-account-issuer=k3s",
	}

	api_server_args, ok := s.config["kube-apiserver-arg"]
	if ok {
		as_string_array, ok := api_server_args.([]string)
		if ok {
			s.config["kube-apiserver-arg"] = append(as_string_array, kube_api_server_args...)
		}
	} else {
		s.config["kube-apiserver-arg"] = kube_api_server_args
	}

	s.addFile("/etc/rancher/k3s/tls/sa-signer-pkcs8.pub", pkcs8_key)
	s.addFile("/etc/rancher/k3s/tls/sa-signer.key", signing_key)
}

func (s *server) addFile(path string, content string) {
	s.extraFiles[path] = content
}

func (s *server) dataDir() string {
	if dir, ok := s.config["data_dir"].(string); ok && dir != "" {
		return dir
	}
	return DATA_DIR
}

// Preinstall implements K3sComponent.
func (s *server) Preinstall(client ssh_client.SSHClient) error {
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

	commands := append(WriteFileCommands(s.binDir+"/k3s-install.sh", installContents),
		[]string{
			fmt.Sprintf("sudo mkdir -p %s", s.dataDir()),
			fmt.Sprintf("sudo mkdir -p %s", CONFIG_DIR),
		}...)

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

// Install implements K3sComponent.
func (s *server) Install(client ssh_client.SSHClient) (err error) {
	flags := []string{
		"INSTALL_K3S_SKIP_START=true",
		fmt.Sprintf("BIN_DIR=%s", s.binDir),
		fmt.Sprintf("INSTALL_K3S_EXEC='--config %s/config.yaml'", CONFIG_DIR),
	}

	// Join existing cluster as HA node
	if s.token != "" {
		flags = append(flags, fmt.Sprintf("K3S_TOKEN=%s", s.token))
	}

	if s.version != "" {
		flags = append(flags, fmt.Sprintf("INSTALL_K3S_VERSION=\"%s\"", s.version))
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

// Uninstall implements K3sServer uninstall.
func (s *server) Uninstall(client ssh_client.SSHClient, kubeconfig string, allowErr ...bool) error {
	return client.RunStream([]string{
		fmt.Sprintf("sudo bash %s/k3s-uninstall.sh", s.binDir),
	})
}

func (s *server) Status(client ssh_client.SSHClient) (bool, error) {
	status, err := systemdStatus("k3s", client)
	if err != nil {
		// Take error as false for status, which should be just as bad
		tflog.Error(s.ctx, fmt.Sprintf("error fetching server status: %s", err.Error()))
	} else if !status {
		tflog.Warn(s.ctx, "k3s server isn't active, dumping journalctl logs to TRACE")
		logs, err := client.Run("sudo journalctl -u k3s")
		if err != nil {
			return false, fmt.Errorf("retrieving journalctl status: %s", err.Error())
		}
		tflog.Trace(s.ctx, logs[0])
	} else {
		tflog.Debug(s.ctx, "k3s server status fetched")
	}

	return status, nil

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
