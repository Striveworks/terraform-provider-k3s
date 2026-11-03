package schemas

import (
	"encoding/base64"
	"fmt"
	"strings"
	"testing"

	"k8s.io/client-go/tools/clientcmd"
	api "k8s.io/client-go/tools/clientcmd/api"
)

const (
	testCAPEM   = "-----BEGIN CERTIFICATE-----\nca-data\n-----END CERTIFICATE-----\n"
	testCertPEM = "-----BEGIN CERTIFICATE-----\nclient-cert-data\n-----END CERTIFICATE-----\n"
	testKeyPEM  = "-----BEGIN EC PRIVATE KEY-----\nclient-key-data\n-----END EC PRIVATE KEY-----\n"
)

// testKubeconfig returns a minimal kubeconfig with the given cluster and user
// names. Certificate fields are base64 encoded in the YAML, as clientcmd
// expects; clientcmd.Load decodes them back to PEM.
func testKubeconfig(clusterName, userName string) string {
	b64 := func(s string) string { return base64.StdEncoding.EncodeToString([]byte(s)) }

	return fmt.Sprintf(`apiVersion: v1
kind: Config
clusters:
- cluster:
    certificate-authority-data: %s
    server: https://127.0.0.1:6443
  name: %s
contexts:
- context:
    cluster: %s
    user: %s
  name: default
current-context: default
users:
- name: %s
  user:
    client-certificate-data: %s
    client-key-data: %s
`, b64(testCAPEM), clusterName, clusterName, userName, userName, b64(testCertPEM), b64(testKeyPEM))
}

func TestBuildClusterAuth(t *testing.T) {
	auth, err := BuildClusterAuth(testKubeconfig("default", "default"))
	if err != nil {
		t.Fatalf("BuildClusterAuth() error = %v", err)
	}

	if got, want := auth.Server.ValueString(), "https://127.0.0.1:6443"; got != want {
		t.Errorf("Server = %q, want %q", got, want)
	}
	if got := auth.CertificateAuthorityData.ValueString(); got != testCAPEM {
		t.Errorf("CertificateAuthorityData = %q, want PEM %q", got, testCAPEM)
	}
	if got := auth.ClientCertificateData.ValueString(); got != testCertPEM {
		t.Errorf("ClientCertificateData = %q, want PEM %q", got, testCertPEM)
	}
	if got := auth.ClientKeyData.ValueString(); got != testKeyPEM {
		t.Errorf("ClientKeyData = %q, want PEM %q", got, testKeyPEM)
	}
}

func TestBuildClusterAuthMissingDefaultCluster(t *testing.T) {
	_, err := BuildClusterAuth(testKubeconfig("prod", "default"))
	if err == nil {
		t.Fatalf("BuildClusterAuth() expected error for kubeconfig without a \"default\" cluster")
	}
	if !strings.Contains(err.Error(), "cluster named \"default\"") {
		t.Errorf("error = %q, want mention of missing \"default\" cluster", err.Error())
	}
}

func TestBuildClusterAuthMissingDefaultUser(t *testing.T) {
	_, err := BuildClusterAuth(testKubeconfig("default", "prod"))
	if err == nil {
		t.Fatalf("BuildClusterAuth() expected error for kubeconfig without a \"default\" user")
	}
	if !strings.Contains(err.Error(), "user named \"default\"") {
		t.Errorf("error = %q, want mention of missing \"default\" user", err.Error())
	}
}

func TestUpdateHost(t *testing.T) {
	auth, err := BuildClusterAuth(testKubeconfig("default", "default"))
	if err != nil {
		t.Fatalf("BuildClusterAuth() error = %v", err)
	}

	if err := auth.UpdateHost("10.0.0.5"); err != nil {
		t.Fatalf("UpdateHost() error = %v", err)
	}

	want := "https://10.0.0.5:6443"
	if got := auth.Server.ValueString(); got != want {
		t.Errorf("Server = %q, want %q", got, want)
	}

	config, err := clientcmd.Load([]byte(auth.KubeConfig()))
	if err != nil {
		t.Fatalf("clientcmd.Load() error = %v", err)
	}
	if got := config.Clusters["default"].Server; got != want {
		t.Errorf("kubeconfig server = %q, want %q", got, want)
	}
}

func TestUpdateHostMissingDefaultCluster(t *testing.T) {
	auth := ClusterAuth{
		config: &api.Config{
			Clusters: map[string]*api.Cluster{
				"prod": {Server: "https://127.0.0.1:6443"},
			},
		},
	}

	if err := auth.UpdateHost("10.0.0.5"); err == nil {
		t.Fatalf("UpdateHost() expected error for kubeconfig without a \"default\" cluster")
	}
}
