package provider

import (
	"context"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"

	"striveworks.us/terraform-provider-k3s/internal/schemas"
	"striveworks.us/terraform-provider-k3s/internal/ssh_client"
)

const testKubeConfig = `apiVersion: v1
clusters:
- cluster:
    certificate-authority-data: Y2EtZGF0YQ==
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
    client-certificate-data: Y2xpZW50LWNlcnQ=
    client-key-data: Y2xpZW50LWtleQ==
`

func TestPopulateKubeConfigData(t *testing.T) {
	ctx := context.Background()
	sshConfig := ssh_client.SSHConfig{
		User:     types.StringValue("root"),
		Host:     types.StringValue("example.com"),
		Port:     types.Int32Value(2222),
		Password: types.StringValue("rootpassword"),
	}
	data := K3sKubeConfigDataModel{
		Hostname: types.StringValue("lb.example.com"),
	}

	if err := populateKubeConfigData(ctx, &data, sshConfig, testKubeConfig); err != nil {
		t.Fatalf("populateKubeConfigData() error = %v", err)
	}

	if got, want := data.K3sURL.ValueString(), "https://lb.example.com:6443"; got != want {
		t.Errorf("K3sURL = %q, want %q", got, want)
	}
	if !strings.Contains(data.KubeConfig.ValueString(), "server: https://lb.example.com:6443") {
		t.Errorf("KubeConfig did not contain overridden server: %s", data.KubeConfig.ValueString())
	}

	var clusterAuth schemas.ClusterAuth
	diags := data.ClusterAuth.As(ctx, &clusterAuth, basetypes.ObjectAsOptions{})
	if diags.HasError() {
		t.Fatalf("ClusterAuth.As() diagnostics = %v", diags)
	}
	if got, want := clusterAuth.Server.ValueString(), "https://lb.example.com:6443"; got != want {
		t.Errorf("clusterAuth.Server = %q, want %q", got, want)
	}
	if got, want := clusterAuth.CertificateAuthorityData.ValueString(), "ca-data"; got != want {
		t.Errorf("clusterAuth.CertificateAuthorityData = %q, want %q", got, want)
	}
	if got, want := clusterAuth.ClientCertificateData.ValueString(), "client-cert"; got != want {
		t.Errorf("clusterAuth.ClientCertificateData = %q, want %q", got, want)
	}
	if got, want := clusterAuth.ClientKeyData.ValueString(), "client-key"; got != want {
		t.Errorf("clusterAuth.ClientKeyData = %q, want %q", got, want)
	}

	var stateAuth ssh_client.SSHConfig
	diags = data.Auth.As(ctx, &stateAuth, basetypes.ObjectAsOptions{})
	if diags.HasError() {
		t.Fatalf("Auth.As() diagnostics = %v", diags)
	}
	if got, want := stateAuth.Host.ValueString(), "example.com"; got != want {
		t.Errorf("stateAuth.Host = %q, want %q", got, want)
	}
	if got, want := stateAuth.Port.ValueInt32(), int32(2222); got != want {
		t.Errorf("stateAuth.Port = %d, want %d", got, want)
	}
}

func TestPopulateKubeConfigDataKeepsExistingServerWithoutHostnameOverride(t *testing.T) {
	ctx := context.Background()
	sshConfig := ssh_client.SSHConfig{
		User:     types.StringValue("root"),
		Host:     types.StringValue("example.com"),
		Port:     types.Int32Value(22),
		Password: types.StringValue("rootpassword"),
	}
	data := K3sKubeConfigDataModel{
		Hostname: types.StringNull(),
	}

	if err := populateKubeConfigData(ctx, &data, sshConfig, testKubeConfig); err != nil {
		t.Fatalf("populateKubeConfigData() error = %v", err)
	}

	if got, want := data.K3sURL.ValueString(), "https://127.0.0.1:6443"; got != want {
		t.Errorf("K3sURL = %q, want %q", got, want)
	}
}

func TestSetEmptyKubeConfigData(t *testing.T) {
	ctx := context.Background()
	sshConfig := ssh_client.SSHConfig{
		User:     types.StringValue("root"),
		Host:     types.StringValue("example.com"),
		Port:     types.Int32Value(22),
		Password: types.StringValue("rootpassword"),
	}
	data := K3sKubeConfigDataModel{}

	setEmptyKubeConfigData(ctx, &data, sshConfig)

	if !data.KubeConfig.IsNull() {
		t.Errorf("KubeConfig should be null, got %#v", data.KubeConfig)
	}
	if !data.K3sURL.IsNull() {
		t.Errorf("K3sURL should be null, got %#v", data.K3sURL)
	}
	if !data.ClusterAuth.IsNull() {
		t.Errorf("ClusterAuth should be null, got %#v", data.ClusterAuth)
	}

	var stateAuth ssh_client.SSHConfig
	diags := data.Auth.As(ctx, &stateAuth, basetypes.ObjectAsOptions{})
	if diags.HasError() {
		t.Fatalf("Auth.As() diagnostics = %v", diags)
	}
	if got, want := stateAuth.Host.ValueString(), "example.com"; got != want {
		t.Errorf("stateAuth.Host = %q, want %q", got, want)
	}
}

func TestNormalizeKubeConfigDataSSHConfig(t *testing.T) {
	sshConfig := ssh_client.SSHConfig{
		Port: types.Int32Null(),
	}

	normalizeKubeConfigDataSSHConfig(&sshConfig)

	if got, want := sshConfig.Port.ValueInt32(), int32(22); got != want {
		t.Errorf("Port = %d, want %d", got, want)
	}
}
