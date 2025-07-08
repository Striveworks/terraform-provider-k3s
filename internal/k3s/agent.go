package k3s

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/joho/godotenv"
	"gopkg.in/yaml.v2"
	"striveworks.us/terraform-provider-k3s/internal/ssh_client"
)

type K3sAgent interface {
	K3sComponent
	Config() map[any]any
	Server() string
}

var _ K3sAgent = &agent{}

type agent struct {
	config  map[any]any
	version *string
	ctx     context.Context
	binDir  string
	token   string
	server  string
}

// Token implements K3sAgent.
func (a *agent) Config() map[any]any {
	return a.config
}

func NewK3sAgentComponent(ctx context.Context, config map[any]any, version *string, token string, server string, binDir string) K3sAgent {
	return &agent{ctx: ctx, config: config, version: version, binDir: binDir, token: token, server: server}
}

func (a *agent) Token() string {
	return a.token
}

func (a *agent) Server() string {
	return a.server
}

// RunInstall implements K3sAgent.
func (a *agent) RunInstall(client ssh_client.SSHClient) error {

	flags := []string{
		"INSTALL_K3S_SKIP_START=true",
		fmt.Sprintf("INSTALL_K3S_EXEC='agent --config %s/config.yaml'", CONFIG_DIR),
		fmt.Sprintf("K3S_URL=%s", a.server),
		fmt.Sprintf("K3S_TOKEN=%s", a.token),
		fmt.Sprintf("BIN_DIR=%s", a.binDir),
	}
	if a.version != nil {
		flags = append(flags, fmt.Sprintf("INSTALL_K3S_VERSION='%s'", *a.version))
	}

	commands := []string{
		fmt.Sprintf("sudo %s bash %s/k3s-install.sh", strings.Join(flags, " "), a.binDir),
		"sudo systemctl daemon-reload",
	}

	if err := client.RunStream(commands); err != nil {
		return err
	}

	if _, err := client.Run("sudo systemctl start k3s-agent"); err != nil {
		log, _ := a.StatusLog(client)
		tflog.Error(a.ctx, log)
		journal, _ := a.Journal(client)
		tflog.Trace(a.ctx, journal)

		return fmt.Errorf("could not start k3s-agent")
	}

	return nil
}

// RunPreReqs implements K3sAgent.
func (a *agent) RunPreReqs(client ssh_client.SSHClient) error {

	if err := client.WaitForReady(); err != nil {
		return err
	}

	configContents, err := yaml.Marshal(a.config)
	if err != nil {
		return err
	}

	installContents, err := ReadInstallScript()
	if err != nil {
		return err
	}

	commands := []string{fmt.Sprintf("sudo mkdir -p %s", a.dataDir()), fmt.Sprintf("sudo mkdir -p %s", CONFIG_DIR)}
	files := []struct {
		path     string
		contents string
	}{
		{a.binDir + "/k3s-install.sh", installContents},
		{CONFIG_DIR + "/config.yaml", base64.StdEncoding.EncodeToString(configContents)},
	}

	for _, file := range files {
		commands = append(commands, WriteFileCommands(file.path, file.contents)...)
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

func (a *agent) Update(client ssh_client.SSHClient) error {
	if err := client.WaitForReady(); err != nil {
		return err
	}

	commands, err := configCommands(a.ctx, a.config)
	if err != nil {
		return err
	}

	return client.RunStream(append(commands, "sudo systemctl restart k3s-agent"))
}

func (a *agent) dataDir() string {
	if dir, ok := a.config["data_dir"].(string); ok && dir != "" {
		return dir
	}
	return DATA_DIR
}

// Retrieve server token.
func (a *agent) getAgentEnv(client ssh_client.SSHClient) (map[string]string, error) {
	file, err := client.ReadFile("/etc/systemd/system/k3s-agent.service.env", false, true)
	if err != nil {
		return nil, err
	}
	tflog.Info(a.ctx, fmt.Sprintf("K3s agent file %v", file))
	return godotenv.Unmarshal(file)
}

func (a *agent) Resync(client ssh_client.SSHClient) (err error) {
	a.config, err = getConfig(client)
	if err != nil {
		return err
	}

	agentEnv, err := a.getAgentEnv(client)
	if err != nil {
		return err
	}

	tflog.Info(a.ctx, fmt.Sprintf("K3s agent env %v", agentEnv))
	if token, ok := agentEnv["K3S_TOKEN"]; ok {
		a.token = token
	} else {
		return fmt.Errorf("could not find agent token")
	}

	if k3sUrl, ok := agentEnv["K3S_URL"]; ok {
		a.server = k3sUrl
	} else {
		return fmt.Errorf("could not find server url")
	}

	return nil
}
