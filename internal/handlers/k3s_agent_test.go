package handlers_test

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
	"striveworks.us/terraform-provider-k3s/internal/handlers"
	"striveworks.us/terraform-provider-k3s/internal/ssh_client"
)

type mockAgent struct {
	status    bool
	statusErr error
	resync    error
	uninstall error
}

func (m mockAgent) Resync(ssh_client.SSHClient) error { return m.resync }
func (m mockAgent) Status(ssh_client.SSHClient) (bool, error) {
	return m.status, m.statusErr
}
func (mockAgent) Server() string { return "" }
func (mockAgent) Token() string  { return "" }
func (m mockAgent) Uninstall(ssh_client.SSHClient, string, ...bool) error {
	return m.uninstall
}

func TestAgentHandlerRead(t *testing.T) {
	t.Parallel()

	t.Run("Bad ssh", func(t *testing.T) {
		var data handlers.AgentClientModel
		err := data.Read(t.Context(), &mockKubeConfigBadSSH{}, &mockAgent{})
		if err == nil {
			t.Errorf("Bad ssh should raise, got nil")
		}
	})

	t.Run("Bad resync", func(t *testing.T) {
		var data handlers.AgentClientModel
		err := data.Read(t.Context(), &mockKubeconfigGoodSSH{}, &mockAgent{resync: fmt.Errorf("error")})
		if err == nil {
			t.Errorf("Bad resync should raise, got nil")
		}
	})

	t.Run("False status", func(t *testing.T) {
		var data handlers.AgentClientModel
		err := data.Read(t.Context(), &mockKubeconfigGoodSSH{}, &mockAgent{status: false})
		if err != nil {
			t.Errorf("Bad status shouldn't raise")
		}
		if !data.Active.Equal(types.BoolValue(false)) {
			t.Errorf("Bad status should result in false active")
		}

	})

	t.Run("good read", func(t *testing.T) {
		var data handlers.AgentClientModel
		err := data.Read(t.Context(), &mockKubeconfigGoodSSH{}, &mockAgent{status: true})
		if err != nil {
			t.Errorf("Bad status shouldn't raise")
		}
		if !data.Active.Equal(types.BoolValue(true)) {
			t.Errorf("Good status should result in true active")
		}
	})
}

func TestAgentHandlerDelete(t *testing.T) {
	t.Parallel()

	t.Run("Bad SSH", func(t *testing.T) {
		var data handlers.AgentClientModel
		err := data.Delete(t.Context(), &mockKubeConfigBadSSH{}, &mockAgent{})
		if err == nil {
			t.Errorf("Bad ssh should raise, got nil")
		}
	})

	t.Run("Good uninstall", func(t *testing.T) {
		var data handlers.AgentClientModel

		err := data.Delete(t.Context(), &mockKubeconfigGoodSSH{}, &mockAgent{})
		if err != nil {
			t.Errorf("Good uninstall shouldn't raise")
		}
	})
}

type mockAgentInstall struct {
	status        bool
	statusErr     error
	preInstallErr error
	installErr    error
}

func (m mockAgentInstall) Status(ssh_client.SSHClient) (bool, error) {
	return m.status, m.statusErr
}
func (m mockAgentInstall) Install(ssh_client.SSHClient) error {
	return m.installErr
}

func (m mockAgentInstall) Preinstall(ssh_client.SSHClient) error {
	return m.preInstallErr
}

func TestAgentHandlerCreate(t *testing.T) {
	t.Parallel()

	t.Run("Bad ssh", func(t *testing.T) {
		data := handlers.AgentClientModel{}
		err := data.Create(t.Context(), &mockKubeConfigBadSSH{}, &mockAgentInstall{})
		if err == nil {
			t.Errorf("Bad ssh should raise, got nil")
		}
	})

	t.Run("Bad preinstall", func(t *testing.T) {
		data := handlers.AgentClientModel{}
		err := data.Create(t.Context(), &mockKubeconfigGoodSSH{}, &mockAgentInstall{preInstallErr: fmt.Errorf("error")})
		if err == nil {
			t.Errorf("Bad preinstall should raise, got nil")
		}
	})

	t.Run("Bad install", func(t *testing.T) {
		var data handlers.AgentClientModel
		err := data.Create(t.Context(), &mockKubeconfigGoodSSH{}, &mockAgentInstall{installErr: fmt.Errorf("error")})
		if err == nil {
			t.Errorf("Bad install should raise, got nil")
		}
	})

	t.Run("False status", func(t *testing.T) {
		var data handlers.AgentClientModel
		ssh := mockSSH{
			hostname: "192.168.1.1",
		}
		err := data.Create(t.Context(), &mockKubeconfigGoodSSH{mockSSH: &ssh}, &mockAgentInstall{status: false})
		if err != nil {
			t.Errorf("Bad status shouldn't raise")
		}
		if !data.Active.Equal(types.BoolValue(false)) {
			t.Errorf("Bad status should result in false active")
		}

	})

	t.Run("good install", func(t *testing.T) {
		var data handlers.AgentClientModel
		ssh := mockSSH{
			hostname: "192.168.1.1",
		}
		err := data.Create(t.Context(), &mockKubeconfigGoodSSH{mockSSH: &ssh}, &mockAgentInstall{status: true})
		if err != nil {
			t.Errorf("Good install shouldn't raise")
		}
		if !data.Active.Equal(types.BoolValue(true)) {
			t.Errorf("Good install should result in true active")
		}
		if !data.Id.Equal(types.StringValue("agent,192.168.1.1")) {
			t.Errorf("Expected `agent,192.168.1.1` for id, got %s", data.Id)
		}
	})
}
