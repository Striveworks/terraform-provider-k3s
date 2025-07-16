package handlers_test

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
	"striveworks.us/terraform-provider-k3s/internal/handlers"
	"striveworks.us/terraform-provider-k3s/internal/ssh_client"
)

type mockServer struct {
	status          bool
	statusErr       error
	resync          error
	uninstall       error
	preinstallError error
	installError    error
}

// Install implements handlers.TServerCreate.
func (m *mockServer) Install(ssh_client.SSHClient) error {
	return m.installError
}

// Preinstall implements handlers.TServerCreate.
func (m *mockServer) Preinstall(ssh_client.SSHClient) error {
	return m.preinstallError
}

// AddOidc implements handlers.TK3sServerRead.
func (m *mockServer) AddOidc(
	audience string,
	issuer string,
	private_key string,
	signing_key string,
) {
}

// KubeConfig implements handlers.TK3sServerRead.
func (m *mockServer) KubeConfig() string {
	return TestMockKubeconfig
}

func (m mockServer) Resync(ssh_client.SSHClient) error { return m.resync }
func (m mockServer) Status(ssh_client.SSHClient) (bool, error) {
	return m.status, m.statusErr
}
func (mockServer) Server() string { return "" }
func (mockServer) Token() string  { return "" }
func (m mockServer) Uninstall(ssh_client.SSHClient, string, ...bool) error {
	return m.uninstall
}

func TestServerRead(t *testing.T) {
	t.Parallel()

	t.Run("Bad Ssh", func(t *testing.T) {
		var data handlers.ServerClientModel
		err := data.Read(t.Context(), &mockKubeConfigBadSSH{}, &mockServer{})
		if err == nil {
			t.Errorf("Bad ssh should raise, got nil")
		}
	})

	t.Run("Bad resync", func(t *testing.T) {
		var data handlers.ServerClientModel
		err := data.Read(t.Context(), &mockKubeconfigGoodSSH{}, &mockServer{resync: fmt.Errorf("error")})
		if err == nil {
			t.Errorf("Bad resync should raise, got nil")
		}
	})

	t.Run("False status", func(t *testing.T) {
		var data handlers.ServerClientModel
		err := data.Read(t.Context(), &mockKubeconfigGoodSSH{}, &mockServer{status: false})
		if err != nil {
			t.Errorf("Bad status shouldn't raise")
		}
		if !data.Active.Equal(types.BoolValue(false)) {
			t.Errorf("Bad status should result in false active")
		}

	})

	t.Run("good read", func(t *testing.T) {
		var data handlers.ServerClientModel
		err := data.Read(t.Context(), &mockKubeconfigGoodSSH{}, &mockServer{status: true})
		if err != nil {
			t.Errorf("Bad status shouldn't raise")
		}
		if !data.Active.Equal(types.BoolValue(true)) {
			t.Errorf("Good status should result in true active")
		}
	})
}

func TestServerHandlerDelete(t *testing.T) {
	t.Parallel()

	t.Run("Bad SSH", func(t *testing.T) {
		var data handlers.ServerClientModel
		err := data.Delete(t.Context(), &mockKubeConfigBadSSH{}, &mockServer{})
		if err == nil {
			t.Errorf("Bad ssh should raise, got nil")
		}
	})

	t.Run("Good uninstall", func(t *testing.T) {
		var data handlers.ServerClientModel

		err := data.Delete(t.Context(), &mockKubeconfigGoodSSH{}, &mockServer{})
		if err != nil {
			t.Errorf("Good uninstall shouldn't raise")
		}
	})
}

func TestServerHandlerCreate(t *testing.T) {
	t.Parallel()

	t.Run("Bad SSH", func(t *testing.T) {
		var data handlers.ServerClientModel
		err := data.Create(t.Context(), &mockKubeConfigBadSSH{}, &mockServer{})
		if err == nil {
			t.Errorf("Bad ssh should raise, got nil")
		}
	})

	t.Run("Bad preinstall", func(t *testing.T) {
		var data handlers.ServerClientModel

		err := data.Create(t.Context(), &mockKubeconfigGoodSSH{}, &mockServer{
			preinstallError: fmt.Errorf("error"),
		})
		if err == nil {
			t.Errorf("Bad prereqs should raise")
		}
	})

	t.Run("Bad install", func(t *testing.T) {
		var data handlers.ServerClientModel

		err := data.Create(t.Context(), &mockKubeconfigGoodSSH{}, &mockServer{
			installError: fmt.Errorf("error"),
		})
		if err == nil {
			t.Errorf("Bad install should raise")
		}
	})

	t.Run("Bad status", func(t *testing.T) {
		var data handlers.ServerClientModel

		err := data.Create(t.Context(), &mockKubeconfigGoodSSH{}, &mockServer{
			statusErr: fmt.Errorf("error"),
		})
		if err == nil {
			t.Errorf("Bad status should raise")
		}
	})

	t.Run("Good install", func(t *testing.T) {
		var data handlers.ServerClientModel
		ssh := mockSSH{}
		err := data.Create(t.Context(), &mockKubeconfigGoodSSH{&ssh}, &mockServer{})
		if err != nil {
			t.Errorf("Good install shouldn't raise")
		}
	})

}
