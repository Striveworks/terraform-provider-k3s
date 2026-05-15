package k3s

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/joho/godotenv"
	"go.yaml.in/yaml/v2"
	"k8s.io/client-go/tools/clientcmd"
	"striveworks.us/terraform-provider-k3s/internal/schemas"
	"striveworks.us/terraform-provider-k3s/internal/ssh_client"
)

var _ K3sComponent = &Server{}

type Server struct {
	Config     string
	Registry   string
	Token      string
	KubeConfig string
	Version    string
	BinDir     string
	ExtraFiles map[string]string
	Env        map[string]string

	// Internal fields to check for
	// correct formatting and config merging
	config   map[any]any
	registry map[any]any
}

func (s *Server) Validate(ctx context.Context) error {

	tflog.Debug(ctx, "Checking if Config is correctly yaml formatted")
	s.config = make(map[any]any)
	if err := yaml.Unmarshal([]byte(s.Config), &s.config); err != nil {
		return fmt.Errorf("parsing config: %s", err.Error())
	}

	s.registry = make(map[any]any)
	if err := yaml.Unmarshal([]byte(s.Registry), &s.registry); err != nil {
		return fmt.Errorf("parsing registry: %s", err.Error())
	}
	if s.BinDir == "" {
		s.BinDir = BIN_DIR
	}

	return nil
}

func (s *Server) WithHa(config schemas.HaConfig) {

	s.config["cluster-init"] = config.ClusterInit.ValueBool()

	if config.Token.ValueString() != "" {
		s.Token = config.Token.ValueString()
	}
	if config.Server.ValueString() != "" {
		s.config["server"] = config.Server.ValueString()
	}
}

func (s *Server) WithOidc(config schemas.OidcConfig) {
	kube_api_server_args := []string{
		fmt.Sprintf("api-audiences=%s", config.Audience.ValueString()),
		"service-account-key-file=/etc/rancher/k3s/tls/sa-signer-pkcs8.pub",
		"service-account-key-file=/var/lib/rancher/k3s/server/tls/service.key",
		"service-account-signing-key-file=/etc/rancher/k3s/tls/sa-signer.key",
		fmt.Sprintf("service-account-issuer=%s", config.Issuer.ValueString()),
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

	s.addFile("/etc/rancher/k3s/tls/sa-signer-pkcs8.pub", config.SigningPKCS8.ValueString())
	s.addFile("/etc/rancher/k3s/tls/sa-signer.key", config.SigningKey.ValueString())
}

// Preinstall implements K3sComponent.
func (s *Server) PreInstall(ctx context.Context, client ssh_client.SSHClient) error {
	if err := client.WaitForReady(); err != nil {
		return err
	}

	cfgCommands, err := configCommands(ctx, s.config)
	if err != nil {
		return err
	}
	regCommands, err := registryCommands(ctx, s.registry)
	if err != nil {
		return err
	}

	extraFileCommands := syncExtraFiles(s.ExtraFiles)

	tflog.Debug(ctx, "Reading install script")
	installContents, err := ReadInstallScript()
	if err != nil {
		return err
	}

	commands := append(
		WriteFileCommands(s.BinDir+"/k3s-install.sh", installContents),
		fmt.Sprintf("sudo mkdir -p %s", CONFIG_DIR),
		fmt.Sprintf("sudo mkdir -p %s", s.dataDir()),
	)

	if s.BinDir != BIN_DIR {
		commands = append(commands, fmt.Sprintf("sudo mkdir -p %s", s.BinDir))
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

// Install implements K3sComponent.
func (s *Server) Install(ctx context.Context, client ssh_client.SSHClient) error {
	commands := []string{
		s.installCommand(),
		"sudo systemctl daemon-reload",
		"sudo systemctl start k3s",
	}

	if err := client.RunStream(commands); err != nil {
		return err
	}

	// If first node on HA, set token
	if s.Token == "" {
		token, err := s.getToken(client)
		if err != nil {
			return err
		}
		s.Token = token
		tflog.MaskMessageStrings(ctx, s.Token)
	}

	// Retrieve kubeconfig
	kubeConfig, err := s.getKubeConfig(client)
	if err != nil {
		return err
	}
	tflog.MaskMessageStrings(ctx, kubeConfig)
	s.KubeConfig = kubeConfig

	return nil
}

func (s *Server) Update(ctx context.Context, client ssh_client.SSHClient) error {
	if err := client.WaitForReady(); err != nil {
		return err
	}

	commands := []string{
		s.installCommand(),
		"sudo systemctl daemon-reload",
		"sudo systemctl restart k3s",
	}

	if err := client.RunStream(commands); err != nil {
		return err
	}

	return nil
}

func (s *Server) Uninstall(ctx context.Context, client ssh_client.SSHClient) error {
	if err := client.WaitForReady(); err != nil {
		return err
	}

	exists, err := k3sServiceExists(client)
	if err != nil {
		return err
	}
	if !exists {
		tflog.Debug(ctx, "k3s service is already absent")
		return nil
	}

	binDir := s.BinDir
	if binDir == "" {
		binDir = BIN_DIR
	}

	if err := client.RunStream([]string{
		fmt.Sprintf("sudo test -f %[1]s/k3s-uninstall.sh && sudo bash %[1]s/k3s-uninstall.sh", binDir),
	}); err != nil {
		return err
	}

	exists, err = k3sServiceExists(client)
	if err != nil {
		return err
	}
	if exists {
		return fmt.Errorf("k3s service still exists after uninstall")
	}

	return nil
}

func (s *Server) Refresh(ctx context.Context, client ssh_client.SSHClient) (exists bool, active bool, err error) {
	exists, err = k3sServiceExists(client)
	if err != nil {
		return false, false, err
	}
	if !exists {
		return false, false, nil
	}

	active, err = k3sServiceActive(client)
	if err != nil {
		return true, false, err
	}

	version, err := k3sBinaryVersion(client, s.BinDir)
	if err != nil {
		return true, active, err
	}
	s.Version = version

	token, err := s.getToken(client)
	if err != nil {
		return true, active, err
	}
	s.Token = token
	tflog.MaskLogStrings(ctx, s.Token)

	kubeConfig, err := s.getKubeConfig(client)
	if err != nil {
		return true, active, err
	}
	s.KubeConfig = kubeConfig
	tflog.MaskLogStrings(ctx, s.KubeConfig)

	return true, active, nil
}

func (s *Server) installCommand() string {
	flags := []string{
		"INSTALL_K3S_SKIP_START=true",
		fmt.Sprintf("BIN_DIR=%s", s.BinDir),
		fmt.Sprintf("INSTALL_K3S_EXEC='--config %s/config.yaml'", CONFIG_DIR),
	}

	// Join existing cluster as HA node or bootstrap with an existing one
	if s.Token != "" {
		flags = append(flags, fmt.Sprintf("K3S_TOKEN=%s", s.Token))
	}

	// Did version get set?
	if s.Version != "" {
		flags = append(flags, fmt.Sprintf("INSTALL_K3S_VERSION=\"%s\"", s.Version))
	}

	for k, v := range s.Env {
		flags = append(flags, fmt.Sprintf("%s=\"%s\"", k, v))
	}

	return fmt.Sprintf("sudo %s bash %s/k3s-install.sh", strings.Join(flags, " "), s.BinDir)
}

func k3sServiceExists(client ssh_client.SSHClient) (bool, error) {
	return k3sSystemdServiceExists(client, "k3s")
}

func k3sServiceActive(client ssh_client.SSHClient) (bool, error) {
	return k3sSystemdServiceActive(client, "k3s")
}

func (s *Server) dataDir() string {
	if dir, ok := s.config["data_dir"].(string); ok && dir != "" {
		return dir
	}
	return DATA_DIR
}

// Retrieve server token.
func (s *Server) getToken(client ssh_client.SSHClient) (string, error) {
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

	return token, nil
}

// Retrieve server token.
func (s *Server) getServerEnv(client ssh_client.SSHClient) (map[string]string, error) {
	file, err := client.ReadFile("/etc/systemd/system/k3s.service.env", false, true)
	if err != nil {
		return nil, err
	}
	return godotenv.Unmarshal(file)
}

func syncExtraFiles(extraFiles map[string]string) (commands []string) {
	for k, v := range extraFiles {
		commands = append(commands,
			fmt.Sprintf("sudo mkdir -p $(sudo realpath $(dirname %s))", k),
		)
		commands = append(commands, WriteFileCommands(k, base64.StdEncoding.EncodeToString([]byte(v)))...)
	}

	return
}

// Retrieve kubeconfig.
func (s *Server) getKubeConfig(client ssh_client.SSHClient) (string, error) {
	kubeconfig, err := client.ReadFile("/etc/rancher/k3s/k3s.yaml", false, true)
	if err != nil {
		return "", fmt.Errorf("could not retrieve kubeconfig: %s", err.Error())
	}

	kubeConfig, err := updateKubeConfig(kubeconfig, client.Host())
	if err != nil {
		return "", fmt.Errorf("could not retrieve server kubeconfig: %s", err.Error())
	}

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

func (s *Server) addFile(path string, content string) {
	if s.ExtraFiles == nil {
		s.ExtraFiles = make(map[string]string)
	}
	s.ExtraFiles[path] = content
}
