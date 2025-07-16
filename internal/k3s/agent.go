package k3s

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/joho/godotenv"
	"gopkg.in/yaml.v2"
	"striveworks.us/terraform-provider-k3s/internal/ssh_client"
)

type AgentConfig interface {
	Config() map[any]any
}

type AgentRegistry interface {
	Registry() map[any]any
}

type AgentServer interface {
	Server() string
}

type Agent interface {
	Component
	AgentServer
	AgentRegistry
	AgentConfig
}

var _ Agent = &agent{}

type agent struct {
	config   map[any]any
	version  string
	ctx      context.Context
	binDir   string
	token    string
	server   string
	registry map[any]any
}

// Token implements K3sAgent.
func (a *agent) Config() map[any]any {
	return a.config
}

// Builds agent for creating and deleting.
func NewK3sAgentComponent(
	ctx context.Context,
	config string,
	registry string,
	version string,
	token string,
	server string,
	binDir string,
) (Agent, error) {
	cfg := make(map[any]any)
	if err := yaml.Unmarshal([]byte(config), &cfg); err != nil {
		return nil, fmt.Errorf("parsing config: %s", err.Error())
	}

	reg := make(map[any]any)
	if err := yaml.Unmarshal([]byte(config), &reg); err != nil {
		return nil, fmt.Errorf("parsing registry: %s", err.Error())
	}

	return &agent{ctx: ctx, config: cfg, registry: reg, version: version, binDir: binDir, token: token, server: server}, nil
}

// Easy constructor for using just uninstall.
func NewK3sAgentUninstall(ctx context.Context, binDir string) Agent {
	return &agent{ctx: ctx, binDir: binDir}
}

func (a *agent) Token() string {
	return a.token
}

func (a *agent) Server() string {
	return a.server
}

func (a *agent) Registry() map[any]any {
	return a.registry
}

// Install implements K3sAgent.
func (a *agent) Install(client ssh_client.SSHClient) error {

	flags := []string{
		"INSTALL_K3S_SKIP_START=true",
		fmt.Sprintf("INSTALL_K3S_EXEC='agent --config %s/config.yaml'", CONFIG_DIR),
		fmt.Sprintf("K3S_URL=%s", a.server),
		fmt.Sprintf("K3S_TOKEN=%s", a.token),
		fmt.Sprintf("BIN_DIR=%s", a.binDir),
	}
	if a.version != "" {
		flags = append(flags, fmt.Sprintf("INSTALL_K3S_VERSION='%s'", a.version))
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

// Preinstall implements K3sAgent.
func (a *agent) Preinstall(client ssh_client.SSHClient) error {

	if err := client.WaitForReady(); err != nil {
		return err
	}

	cfgCommands, err := configCommands(a.ctx, a.config)
	if err != nil {
		return err
	}

	installContents, err := ReadInstallScript()
	if err != nil {
		return err
	}

	regCommands, err := registryCommands(a.ctx, a.registry)
	if err != nil {
		return err
	}

	commands := append(WriteFileCommands(a.binDir+"/k3s-install.sh", installContents),
		[]string{
			fmt.Sprintf("sudo mkdir -p %s", a.dataDir()),
			fmt.Sprintf("sudo mkdir -p %s", CONFIG_DIR),
		}...)

	// Write config file
	commands = append(commands, cfgCommands...)

	if len(regCommands) > 0 {
		commands = append(commands, regCommands...)
	}

	return client.RunStream(commands)
}

// Uninstall implements K3sAgent.
func (a *agent) Uninstall(client ssh_client.SSHClient, kubeconfig string, allowErr ...bool) error {
	hostname, err := client.Hostname()
	if err != nil {
		return err
	}
	if err := deleteNode(a.ctx, kubeconfig, hostname); err != nil {
		allowErr := len(allowErr) > 0 && allowErr[0]
		if !allowErr {
			return err
		}
		tflog.Warn(a.ctx, fmt.Sprintf("error deleting node via kubectl: %s", err.Error()))

	}
	return client.RunStream([]string{fmt.Sprintf("sudo bash %s/k3s-agent-uninstall.sh", a.binDir)})
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
	status, err := systemdStatus("k3s-agent", client)
	if err != nil {
		// Take error as false for status, which should be just as bad
		tflog.Error(a.ctx, fmt.Sprintf("error fetching agent agent status: %s", err.Error()))
	} else if !status {
		tflog.Warn(a.ctx, "k3s agent isn't active, dumping journalctl logs to TRACE")
		logs, err := client.Run("sudo journalctl -u k3s-agent")
		if err != nil {
			return false, fmt.Errorf("retrieving journalctl status: %s", err.Error())
		}
		tflog.Trace(a.ctx, logs[0])
	} else {
		tflog.Debug(a.ctx, "k3s agent status fetched")
	}

	return status, nil
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
