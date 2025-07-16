package handlers_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"striveworks.us/terraform-provider-k3s/internal/handlers"
	"striveworks.us/terraform-provider-k3s/internal/ssh_client"
)

const TestMockKubeconfig = `apiVersion: v1
clusters:
- cluster:
    certificate-authority-data: LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSUJkekNDQVIyZ0F3SUJBZ0lCQURBS0JnZ3Foa2pPUFFRREFqQWpNU0V3SHdZRFZRUUREQmhyTTNNdGMyVnkKZG1WeUxXTmhRREUzTkRneE9EVTFORFF3SGhjTk1qVXdOVEkxTVRVd05UUTBXaGNOTXpVd05USXpNVFV3TlRRMApXakFqTVNFd0h3WURWUVFEREJock0zTXRjMlZ5ZG1WeUxXTmhRREUzTkRneE9EVTFORFF3V1RBVEJnY3Foa2pPClBRSUJCZ2dxaGtqT1BRTUJCd05DQUFUa0w1dHF1czdESXRpVklCN1FFcG1nQ1Q1Z3R2NUJVRjV5SHJQYWZMaGsKdG1ZWWpkTXE0eldlSmJPLzI1QWZIU1QyT09EazV1bzNWb3dRbk83eEdzRkZvMEl3UURBT0JnTlZIUThCQWY4RQpCQU1DQXFRd0R3WURWUjBUQVFIL0JBVXdBd0VCL3pBZEJnTlZIUTRFRmdRVVVTNlVycmhWWDh5b0FuQ0dMOEJLCmYyZ0VNZDB3Q2dZSUtvWkl6ajBFQXdJRFNBQXdSUUloQUw1YnoxbkVUbnZURFh1anZmb0JuaWUzbGdQM3lFSUkKTXJUYXQzRy83WE5KQWlBeS9PV2E3TmVzb0VrR3RpU3UzTW5OMyszUzhZUHNab3NLR20xZEVpdWEvZz09Ci0tLS0tRU5EIENFUlRJRklDQVRFLS0tLS0K
    server: https://127.0.0.1:6443
  name: default
contexts:
- context:
    cluster: default
    user: default
  name: default
current-context: default
kind: Config
preferences: {}
users:
- name: default
  user:
    client-certificate-data: LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSUJrVENDQVRlZ0F3SUJBZ0lJT1R2MU5KRWhOSnd3Q2dZSUtvWkl6ajBFQXdJd0l6RWhNQjhHQTFVRUF3d1kKYXpOekxXTnNhV1Z1ZEMxallVQXhOelE0TVRnMU5UUTBNQjRYRFRJMU1EVXlOVEUxTURVME5Gb1hEVEkyTURVeQpOVEUxTURVME5Gb3dNREVYTUJVR0ExVUVDaE1PYzNsemRHVnRPbTFoYzNSbGNuTXhGVEFUQmdOVkJBTVRESE41CmMzUmxiVHBoWkcxcGJqQlpNQk1HQnlxR1NNNDlBZ0VHQ0NxR1NNNDlBd0VIQTBJQUJOQ3cyVmpqVjRBYjd6OUsKeXl5SlF4MmFXaUdpMElFUnRlNjRCZVBLM3RTUkVSdGJFdnE4Uk1aZVhXS0lTMVhSMTZrdjNKNE5sM2J1WFlKZAppZS9YSVB1alNEQkdNQTRHQTFVZER3RUIvd1FFQXdJRm9EQVRCZ05WSFNVRUREQUtCZ2dyQmdFRkJRY0RBakFmCkJnTlZIU01FR0RBV2dCU2EvK2pGbEl5c0hjRlhGN3FKTUluL0crTEt1VEFLQmdncWhrak9QUVFEQWdOSUFEQkYKQWlFQXV3aXRTU1NLeUZFaW5VSEZVZXFwV3F3bXIwTlFCMDhoU2ZmekxVazdkMFlDSURYRG1BbUZrczlsYUpNTApvanBXVkNTc3g4bW9XSXFSYzFuSWpGR09HcEJ6Ci0tLS0tRU5EIENFUlRJRklDQVRFLS0tLS0KLS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSUJkekNDQVIyZ0F3SUJBZ0lCQURBS0JnZ3Foa2pPUFFRREFqQWpNU0V3SHdZRFZRUUREQmhyTTNNdFkyeHAKWlc1MExXTmhRREUzTkRneE9EVTFORFF3SGhjTk1qVXdOVEkxTVRVd05UUTBXaGNOTXpVd05USXpNVFV3TlRRMApXakFqTVNFd0h3WURWUVFEREJock0zTXRZMnhwWlc1MExXTmhRREUzTkRneE9EVTFORFF3V1RBVEJnY3Foa2pPClBRSUJCZ2dxaGtqT1BRTUJCd05DQUFSNHk0Qmt5OFB5MVYxVjhnY0FVcU9KMUg0N0RnYWJjQVdsdldxc21ya2cKaUZxN2NOdGJTZThWK3NDdTVtcWtpa2dHcm1EUTgyRFpoQXJNeGhVSEVIZVRvMEl3UURBT0JnTlZIUThCQWY4RQpCQU1DQXFRd0R3WURWUjBUQVFIL0JBVXdBd0VCL3pBZEJnTlZIUTRFRmdRVW12L294WlNNckIzQlZ4ZTZpVENKCi94dml5cmt3Q2dZSUtvWkl6ajBFQXdJRFNBQXdSUUloQVBWbFI5d1JmVkQ1aDM5citsLytHVVZJNFJNV0dNN0YKVllrQVVlRHFLYmlOQWlCb2RmK251VzQ2L1QwdnNVemZ5V2FCVTgyNkxHTEs5bWt4VE9HMDYyV2p6Zz09Ci0tLS0tRU5EIENFUlRJRklDQVRFLS0tLS0K
    client-key-data: LS0tLS1CRUdJTiBFQyBQUklWQVRFIEtFWS0tLS0tCk1IY0NBUUVFSUJZcWdIU2hQNkNudUlpemJiK21KaXVsNE1aNUdJNStSSG54QU0rdnc2dnNvQW9HQ0NxR1NNNDkKQXdFSG9VUURRZ0FFMExEWldPTlhnQnZ2UDByTExJbERIWnBhSWFMUWdSRzE3cmdGNDhyZTFKRVJHMXNTK3J4RQp4bDVkWW9oTFZkSFhxUy9jbmcyWGR1NWRnbDJKNzljZyt3PT0KLS0tLS1FTkQgRUMgUFJJVkFURSBLRVktLS0tLQo=
`

const expectedCA = `-----BEGIN CERTIFICATE-----\nMIIBdzCCAR2gAwIBAgIBADAKBggqhkjOPQQDAjAjMSEwHwYDVQQDDBhrM3Mtc2Vy\ndmVyLWNhQDE3NDgxODU1NDQwHhcNMjUwNTI1MTUwNTQ0WhcNMzUwNTIzMTUwNTQ0\nWjAjMSEwHwYDVQQDDBhrM3Mtc2VydmVyLWNhQDE3NDgxODU1NDQwWTATBgcqhkjO\nPQIBBggqhkjOPQMBBwNCAATkL5tqus7DItiVIB7QEpmgCT5gtv5BUF5yHrPafLhk\ntmYYjdMq4zWeJbO/25AfHST2OODk5uo3VowQnO7xGsFFo0IwQDAOBgNVHQ8BAf8E\nBAMCAqQwDwYDVR0TAQH/BAUwAwEB/zAdBgNVHQ4EFgQUUS6UrrhVX8yoAnCGL8BK\nf2gEMd0wCgYIKoZIzj0EAwIDSAAwRQIhAL5bz1nETnvTDXujvfoBnie3lgP3yEII\nMrTat3G/7XNJAiAy/OWa7NesoEkGtiSu3MnN3+3S8YPsZosKGm1dEiua/g==\n-----END CERTIFICATE-----`

const expectedCD = `-----BEGIN CERTIFICATE-----\nMIIBkTCCATegAwIBAgIIOTv1NJEhNJwwCgYIKoZIzj0EAwIwIzEhMB8GA1UEAwwY\nazNzLWNsaWVudC1jYUAxNzQ4MTg1NTQ0MB4XDTI1MDUyNTE1MDU0NFoXDTI2MDUy\nNTE1MDU0NFowMDEXMBUGA1UEChMOc3lzdGVtOm1hc3RlcnMxFTATBgNVBAMTDHN5\nc3RlbTphZG1pbjBZMBMGByqGSM49AgEGCCqGSM49AwEHA0IABNCw2VjjV4Ab7z9K\nyyyJQx2aWiGi0IERte64BePK3tSRERtbEvq8RMZeXWKIS1XR16kv3J4Nl3buXYJd\nie/XIPujSDBGMA4GA1UdDwEB/wQEAwIFoDATBgNVHSUEDDAKBggrBgEFBQcDAjAf\nBgNVHSMEGDAWgBSa/+jFlIysHcFXF7qJMIn/G+LKuTAKBggqhkjOPQQDAgNIADBF\nAiEAuwitSSSKyFEinUHFUeqpWqwmr0NQB08hSffzLUk7d0YCIDXDmAmFks9laJML\nojpWVCSsx8moWIqRc1nIjFGOGpBz\n-----END CERTIFICATE-----\n-----BEGIN CERTIFICATE-----\nMIIBdzCCAR2gAwIBAgIBADAKBggqhkjOPQQDAjAjMSEwHwYDVQQDDBhrM3MtY2xp\nZW50LWNhQDE3NDgxODU1NDQwHhcNMjUwNTI1MTUwNTQ0WhcNMzUwNTIzMTUwNTQ0\nWjAjMSEwHwYDVQQDDBhrM3MtY2xpZW50LWNhQDE3NDgxODU1NDQwWTATBgcqhkjO\nPQIBBggqhkjOPQMBBwNCAAR4y4Bky8Py1V1V8gcAUqOJ1H47DgabcAWlvWqsmrkg\niFq7cNtbSe8V+sCu5mqkikgGrmDQ82DZhArMxhUHEHeTo0IwQDAOBgNVHQ8BAf8E\nBAMCAqQwDwYDVR0TAQH/BAUwAwEB/zAdBgNVHQ4EFgQUmv/oxZSMrB3BVxe6iTCJ\n/xviyrkwCgYIKoZIzj0EAwIDSAAwRQIhAPVlR9wRfVD5h39r+l/+GUVI4RMWGM7F\nVYkAUeDqKbiNAiBodf+nuW46/T0vsUzfyWaBU826LGLK9mkxTOG062Wjzg==\n-----END CERTIFICATE-----`

const expectedCK = `-----BEGIN EC PRIVATE KEY-----\nMHcCAQEEIBYqgHShP6CnuIizbb+mJiul4MZ5GI5+RHnxAM+vw6vsoAoGCCqGSM49\nAwEHoUQDQgAE0LDZWONXgBvvP0rLLIlDHZpaIaLQgRG17rgF48re1JERG1sS+rxE\nxl5dYohLVdHXqS/cng2Xdu5dgl2J79cg+w==\n-----END EC PRIVATE KEY-----`

type mockKubeConfigBadSSH struct{}

func (mockKubeConfigBadSSH) SshClient(context.Context) (ssh_client.SSHClient, error) {
	return nil, fmt.Errorf("error")
}
func (mockKubeConfigBadSSH) ToObject(ctx context.Context) basetypes.ObjectValue {
	return basetypes.ObjectValue{}
}

type mockKubeconfigGoodSSH struct {
	mockSSH ssh_client.SSHClient
}

func (m mockKubeconfigGoodSSH) SshClient(context.Context) (ssh_client.SSHClient, error) {
	return m.mockSSH, nil
}
func (mockKubeconfigGoodSSH) ToObject(ctx context.Context) basetypes.ObjectValue {
	var data handlers.ClusterAuth
	return data.ToObject(ctx)
}

type mockKubeconfigBadResync struct{}

func (mockKubeconfigBadResync) KubeConfig() string                { return "" }
func (mockKubeconfigBadResync) Resync(ssh_client.SSHClient) error { return fmt.Errorf("") }

type mockKubeconfigGoodResync struct{}

func (mockKubeconfigGoodResync) KubeConfig() string                { return TestMockKubeconfig }
func (mockKubeconfigGoodResync) Resync(ssh_client.SSHClient) error { return nil }

func TestKubecConfigHandlerRead(t *testing.T) {
	t.Parallel()

	t.Run("Bad ssh", func(t *testing.T) {
		data := handlers.K3sKubeConfig{}
		err := data.Read(t.Context(), &mockKubeConfigBadSSH{}, &mockKubeconfigBadResync{})
		if err == nil {
			t.Errorf("Bad ssh should raise, got nil")
		}
	})

	t.Run("Bad resync without allow", func(t *testing.T) {
		data := handlers.K3sKubeConfig{}
		err := data.Read(t.Context(), &mockKubeconfigGoodSSH{}, &mockKubeconfigBadResync{})
		if err == nil {
			t.Errorf("Bad resync should raise, got nil")
		}
	})

	t.Run("Bad resync with allow", func(t *testing.T) {
		data := handlers.K3sKubeConfig{
			AllowEmpty: types.BoolValue(true),
		}
		err := data.Read(t.Context(), &mockKubeconfigGoodSSH{}, &mockKubeconfigBadResync{})
		if err != nil {
			t.Errorf("Bad resync shouldn't raise, got nil")
		}

		if !data.Auth.IsNull() {
			t.Errorf("auth object should null")
		}
		if !data.ClusterAuth.IsNull() {
			t.Errorf("cluster auth object should null")
		}
	})

	t.Run("Good resync", func(t *testing.T) {
		data := handlers.K3sKubeConfig{
			AllowEmpty: types.BoolValue(true),
		}
		err := data.Read(t.Context(), &mockKubeconfigGoodSSH{}, &mockKubeconfigGoodResync{})
		if err != nil {
			t.Errorf("Bad resync shouldn't raise, got err: %s", err.Error())
		}

		if data.Auth.IsNull() {
			t.Errorf("auth object should not be null")
		}
		if data.ClusterAuth.IsNull() {
			t.Errorf("cluster auth object should not be null")
		}

		if !data.KubeConfig.Equal(types.StringValue(TestMockKubeconfig)) {
			t.Errorf("kubeconfig is not corect")
		}

		got := data.ClusterAuth.Attributes()["client_certificate_data"]
		if got.Equal(types.StringValue(expectedCD)) {
			t.Errorf("bad client_certificate_data. Expected %s got %s", expectedCD, got)
		}
		got = data.ClusterAuth.Attributes()["client_key_data"]
		if got.Equal(types.StringValue(expectedCK)) {
			t.Errorf("bad client_key_data. Expected %s got %s", expectedCD, got)
		}
		got = data.ClusterAuth.Attributes()["certificate_authority_data"]
		if got.Equal(types.StringValue(expectedCA)) {
			t.Errorf("bad certificate_authority_data. Expected %s got %s", expectedCD, got)
		}
	})

	t.Run("Good resync with hostname", func(t *testing.T) {
		data := handlers.K3sKubeConfig{
			AllowEmpty: types.BoolValue(true),
			Hostname:   types.StringValue("mylb-name"),
		}
		err := data.Read(t.Context(), &mockKubeconfigGoodSSH{}, &mockKubeconfigGoodResync{})
		if err != nil {
			t.Errorf("Bad resync shouldn't raise, got err: %s", err.Error())
		}
	})
}
