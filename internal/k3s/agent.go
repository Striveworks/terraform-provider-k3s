package k3s

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/joho/godotenv"
	"go.yaml.in/yaml/v2"
	"striveworks.us/terraform-provider-k3s/internal/ssh_client"
)

var _ K3sComponent = &Agent{}

type Agent struct {
	Config     string
	Registry   string
	Token      string
	Version    string
	BinDir     string
	ExtraFiles map[string]string
	Env        map[string]string
	Server     string

	// Internal fields to check for
	// correct formatting and config merging
	config   map[any]any
	registry map[any]any
}

// Install implements [K3sComponent].
func (a *Agent) Install(ctx context.Context, client ssh_client.SSHClient) error {
	if a.Token != "" {
		tflog.MaskMessageStrings(ctx, a.Token)
	}

	commands := []string{
		a.installCommand(),
		"sudo systemctl daemon-reload",
		"sudo systemctl start k3s-agent",
	}

	if err := client.RunStream(commands); err != nil {
		return err
	}

	return nil
}

// PreInstall implements [K3sComponent].
func (a *Agent) PreInstall(ctx context.Context, client ssh_client.SSHClient) error {
	if err := client.WaitForReady(); err != nil {
		return err
	}

	cfgCommands, err := configCommands(ctx, a.config)
	if err != nil {
		return err
	}
	regCommands, err := registryCommands(ctx, a.registry)
	if err != nil {
		return err
	}
	extraFileCommands := syncExtraFiles(a.ExtraFiles)

	tflog.Debug(ctx, "Reading install script")
	installContents, err := ReadInstallScript()
	if err != nil {
		return err
	}

	commands := []string{
		fmt.Sprintf("sudo mkdir -p %s", CONFIG_DIR),
		fmt.Sprintf("sudo mkdir -p %s", a.dataDir()),
	}

	if a.BinDir != BIN_DIR {
		commands = append(commands, fmt.Sprintf("sudo mkdir -p %s", a.BinDir))
	}

	commands = append(commands, WriteFileCommands(a.BinDir+"/k3s-install.sh", installContents)...)
	commands = append(commands, cfgCommands...)

	if len(regCommands) > 0 {
		commands = append(commands, regCommands...)
	}
	if len(extraFileCommands) > 0 {
		commands = append(commands, extraFileCommands...)
	}

	return client.RunStream(commands)
}

// Refresh implements [K3sComponent].
func (a *Agent) Refresh(ctx context.Context, client ssh_client.SSHClient) (exists bool, active bool, err error) {
	exists, err = k3sAgentServiceExists(client)
	if err != nil {
		return false, false, err
	}
	if !exists {
		return false, false, nil
	}

	active, err = k3sAgentServiceActive(client)
	if err != nil {
		return true, false, err
	}

	version, err := k3sBinaryVersion(client, a.BinDir)
	if err != nil {
		return true, active, err
	}
	a.Version = version

	agentEnv, err := a.getAgentEnv(client)
	if err != nil {
		return true, active, err
	}

	token, ok := agentEnv["K3S_TOKEN"]
	if !ok || token == "" {
		return true, active, fmt.Errorf("could not find agent token")
	}
	a.Token = token
	tflog.MaskLogStrings(ctx, a.Token)

	server, ok := agentEnv["K3S_URL"]
	if !ok || server == "" {
		return true, active, fmt.Errorf("could not find server url")
	}
	a.Server = server

	return true, active, nil
}

// Uninstall implements [K3sComponent].
func (a *Agent) Uninstall(ctx context.Context, client ssh_client.SSHClient) error {
	if err := client.WaitForReady(); err != nil {
		return err
	}

	exists, err := k3sAgentServiceExists(client)
	if err != nil {
		return err
	}
	if !exists {
		tflog.Debug(ctx, "k3s-agent service is already absent")
		return nil
	}

	binDir := a.BinDir
	if binDir == "" {
		binDir = BIN_DIR
	}

	if err := client.RunStream([]string{
		fmt.Sprintf("sudo test -f %[1]s/k3s-agent-uninstall.sh && sudo bash %[1]s/k3s-agent-uninstall.sh", binDir),
	}); err != nil {
		return err
	}

	exists, err = k3sAgentServiceExists(client)
	if err != nil {
		return err
	}
	if exists {
		return fmt.Errorf("k3s-agent service still exists after uninstall")
	}

	return nil
}

func (a *Agent) Validate(ctx context.Context) error {

	tflog.Debug(ctx, "Checking if Config is correctly yaml formatted")
	a.config = make(map[any]any)
	if err := yaml.Unmarshal([]byte(a.Config), &a.config); err != nil {
		return fmt.Errorf("parsing config: %s", err.Error())
	}

	a.registry = make(map[any]any)
	if err := yaml.Unmarshal([]byte(a.Registry), &a.registry); err != nil {
		return fmt.Errorf("parsing registry: %s", err.Error())
	}
	if a.BinDir == "" {
		a.BinDir = BIN_DIR
	}

	return nil
}

func (a *Agent) Update(ctx context.Context, client ssh_client.SSHClient) error {
	if err := client.WaitForReady(); err != nil {
		return err
	}

	if a.Token != "" {
		tflog.MaskMessageStrings(ctx, a.Token)
	}

	commands := []string{
		a.installCommand(),
		"sudo systemctl daemon-reload",
		"sudo systemctl restart k3s-agent",
	}

	if err := client.RunStream(commands); err != nil {
		return err
	}

	return nil
}

func (a *Agent) installCommand() string {
	flags := []string{
		"INSTALL_K3S_SKIP_START=true",
		fmt.Sprintf("BIN_DIR=%s", a.BinDir),
		fmt.Sprintf("INSTALL_K3S_EXEC='agent --config %s/config.yaml'", CONFIG_DIR),
		fmt.Sprintf("K3S_URL=%s", a.Server),
		fmt.Sprintf("K3S_TOKEN=%s", a.Token),
	}

	if a.Version != "" {
		flags = append(flags, fmt.Sprintf("INSTALL_K3S_VERSION=\"%s\"", a.Version))
	}

	for k, v := range a.Env {
		flags = append(flags, fmt.Sprintf("%s=\"%s\"", k, v))
	}

	return fmt.Sprintf("sudo %s bash %s/k3s-install.sh", strings.Join(flags, " "), a.BinDir)
}

func (a *Agent) dataDir() string {
	if dir, ok := a.config["data_dir"].(string); ok && dir != "" {
		return dir
	}
	return DATA_DIR
}

func k3sAgentServiceExists(client ssh_client.SSHClient) (bool, error) {
	return k3sSystemdServiceExists(client, "k3s-agent")
}

func k3sAgentServiceActive(client ssh_client.SSHClient) (bool, error) {
	return k3sSystemdServiceActive(client, "k3s-agent")
}

func (a *Agent) getAgentEnv(client ssh_client.SSHClient) (map[string]string, error) {
	file, err := client.ReadFile("/etc/systemd/system/k3s-agent.service.env", false, true)
	if err != nil {
		return nil, err
	}
	return godotenv.Unmarshal(file)
}
