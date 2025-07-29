package handlers

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"striveworks.us/terraform-provider-k3s/internal/k3s"
)

type AgentClientModel struct {
	Auth types.Object `tfsdk:"auth"`
	// Connection
	Server types.String `tfsdk:"server"`
	BinDir types.String `tfsdk:"bin_dir"`
	// Configs
	KubeConfig     types.String `tfsdk:"kubeconfig"`
	K3sRegistry    types.String `tfsdk:"registry"`
	K3sConfig      types.String `tfsdk:"config"`
	Token          types.String `tfsdk:"token"`
	AllowDeleteErr types.Bool   `tfsdk:"allow_delete_err"`
	// Outputs
	Id     types.String `tfsdk:"id"`
	Active types.Bool   `tfsdk:"active"`

	version string
}

func (a *AgentClientModel) ToAgent(ctx context.Context) (k3s.Agent, error) {
	return k3s.NewK3sAgentComponent(
		ctx,
		a.K3sConfig.ValueString(),
		a.K3sRegistry.ValueString(),
		a.version,
		a.Token.ValueString(),
		a.Server.ValueString(),
		a.BinDir.ValueString(),
	)
}

// Hides version so terraform doesn't expose it on the model.
func (a *AgentClientModel) SetVersion(version *string) {
	if version != nil {
		a.version = *version
	}
}

type TAgentRead interface {
	K3sTypeSSH
	K3sTypeToObject
}

type TK3sAgentRead interface {
	k3s.ComponentResync
	k3s.ComponentStatus
	k3s.ComponentToken
	k3s.AgentServer
}

func (a *AgentClientModel) Read(
	ctx context.Context,
	auth TAgentRead,
	agent TK3sAgentRead,
) error {
	sshClient, err := auth.SshClient(ctx)
	if err != nil {
		return fmt.Errorf("creating ssh config: %s", err.Error())
	}
	tflog.Debug(ctx, "k3s agent ssh client created")

	if err := agent.Resync(sshClient); err != nil {
		return fmt.Errorf("error resyncing agent: %s", err.Error())
	}
	tflog.Debug(ctx, "k3s agent resynced")

	status, err := agent.Status(sshClient)
	if err != nil {
		return fmt.Errorf("fetching status or status logs: %s", err.Error())
	}

	a.Active = types.BoolValue(status)
	a.Server = types.StringValue(agent.Server())
	a.Token = types.StringValue(agent.Token())

	return nil
}

func (a *AgentClientModel) Delete(
	ctx context.Context,
	auth K3sTypeSSH,
	agent k3s.ComponentUninstall,
) error {
	sshClient, err := auth.SshClient(ctx)
	if err != nil {
		return fmt.Errorf("creating ssh config: %s", err.Error())
	}
	tflog.Debug(ctx, "k3s agent ssh client created")

	if err := agent.Uninstall(sshClient, a.KubeConfig.ValueString(), a.AllowDeleteErr.ValueBool()); err != nil {
		return fmt.Errorf("creating uninstall k3s-agent: %s", err.Error())
	}
	tflog.Debug(ctx, "k3s agent uninstalled")

	return nil
}

type TK3sAgentCreate interface {
	k3s.ComponentPreInstall
	k3s.ComponentInstall
	k3s.ComponentStatus
}

func (a *AgentClientModel) Create(
	ctx context.Context,
	auth K3sTypeSSH,
	agent TK3sAgentCreate,
) error {
	sshClient, err := auth.SshClient(ctx)
	if err != nil {
		return fmt.Errorf("creating ssh config: %s", err.Error())
	}
	tflog.Debug(ctx, "k3s agent ssh client created")

	if err := agent.Preinstall(sshClient); err != nil {
		return fmt.Errorf("k3s agent performing preinstall: %s", err.Error())
	}
	tflog.Debug(ctx, "k3s agent preinstall success")

	if err := agent.Install(sshClient); err != nil {
		return fmt.Errorf("k3s agent performing install: %s", err.Error())
	}
	tflog.Debug(ctx, "k3s agent install success")

	status, err := agent.Status(sshClient)
	if err != nil {
		return fmt.Errorf("fetching status or status logs: %s", err.Error())
	}
	a.Active = types.BoolValue(status)
	a.Id = types.StringValue(fmt.Sprintf("agent,%s", sshClient.HostnameOrIpAddress()))

	return nil
}

type TK3sAgentUpdate interface {
	k3s.ComponentPreInstall
	k3s.ComponentUpdate
	k3s.ComponentStatus
}

func (existing *AgentClientModel) Update(
	ctx context.Context,
	inc AgentClientModel,
	auth K3sTypeSSH,
	agent TK3sAgentUpdate,
) error {
	sshClient, err := auth.SshClient(ctx)
	if err != nil {
		return fmt.Errorf("creating ssh config: %s", err.Error())
	}
	tflog.Debug(ctx, "k3s agent ssh client created")

	if existing.K3sConfig.Equal(inc.K3sConfig) && existing.K3sRegistry.Equal(inc.K3sRegistry) {
		tflog.Debug(ctx, "No change is needed, only supporting config and registry updates")
		return nil
	}

	if err := agent.Preinstall(sshClient); err != nil {
		return fmt.Errorf("k3s agent updating: %s", err.Error())
	}

	if err := agent.Update(sshClient); err != nil {
		return fmt.Errorf("k3s agent updating: %s", err.Error())
	}

	// Now check status after update
	status, err := agent.Status(sshClient)
	if err != nil {
		return fmt.Errorf("fetching status or status logs: %s", err.Error())
	}

	existing.Active = types.BoolValue(status)
	existing.K3sRegistry = inc.K3sRegistry
	existing.K3sConfig = inc.K3sConfig

	return nil
}
