package k3s

import (
	"context"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
	"striveworks.us/terraform-provider-k3s/internal/schemas"
)

func TestServerWithHaClusterInit(t *testing.T) {
	server := Server{
		Config: "disable-agent: true\n",
	}

	if err := server.Validate(context.Background()); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}

	server.WithHa(schemas.HaConfig{
		ClusterInit: types.BoolValue(true),
		Token:       types.StringNull(),
		Server:      types.StringNull(),
	})

	clusterInit, ok := server.config["cluster-init"].(bool)
	if !ok {
		t.Fatalf("cluster-init was not written as a bool: %#v", server.config["cluster-init"])
	}
	if !clusterInit {
		t.Errorf("cluster-init = false, want true")
	}

	disableAgent, ok := server.config["disable-agent"].(bool)
	if !ok {
		t.Fatalf("disable-agent was not preserved as a bool: %#v", server.config["disable-agent"])
	}
	if !disableAgent {
		t.Errorf("disable-agent = false, want true")
	}

	if server.Token != "" {
		t.Errorf("Token = %q, want empty token for cluster-init", server.Token)
	}
	if _, ok := server.config["server"]; ok {
		t.Errorf("server config was set for cluster-init: %#v", server.config["server"])
	}
}

func TestServerWithHaJoinCluster(t *testing.T) {
	server := Server{
		Config: "write-kubeconfig-mode: \"0600\"\n",
	}

	if err := server.Validate(context.Background()); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}

	server.WithHa(schemas.HaConfig{
		ClusterInit: types.BoolValue(false),
		Token:       types.StringValue("join-token"),
		Server:      types.StringValue("https://10.0.0.1:6443"),
	})

	clusterInit, ok := server.config["cluster-init"].(bool)
	if !ok {
		t.Fatalf("cluster-init was not written as a bool: %#v", server.config["cluster-init"])
	}
	if clusterInit {
		t.Errorf("cluster-init = true, want false")
	}

	if got := server.Token; got != "join-token" {
		t.Errorf("Token = %q, want %q", got, "join-token")
	}
	if got := server.config["server"]; got != "https://10.0.0.1:6443" {
		t.Errorf("server config = %#v, want %q", got, "https://10.0.0.1:6443")
	}

	command := server.installCommand()
	if !strings.Contains(command, "K3S_TOKEN='join-token'") {
		t.Errorf("installCommand() = %q, want K3S_TOKEN flag", command)
	}
	if !strings.Contains(command, "INSTALL_K3S_BIN_DIR='/usr/local/bin'") {
		t.Errorf("installCommand() = %q, want default INSTALL_K3S_BIN_DIR", command)
	}
}

func TestServerWithHaExternalDatastore(t *testing.T) {
	server := Server{
		Config: "datastore-endpoint: postgres://k3s:secret@db.example.com:5432/k3s\n",
	}

	if err := server.Validate(context.Background()); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}

	server.WithHa(schemas.HaConfig{
		ClusterInit: types.BoolValue(false),
		Token:       types.StringValue("join-token"),
		Server:      types.StringNull(),
	})

	if _, ok := server.config["server"]; ok {
		t.Errorf("server config was set without highly_available.server: %#v", server.config["server"])
	}
	if got := server.Token; got != "join-token" {
		t.Errorf("Token = %q, want %q", got, "join-token")
	}
}

func TestServerWithOidcMergesExistingApiServerArgs(t *testing.T) {
	// Round-trip through Validate so the existing value is a []any,
	// exactly as yaml.Unmarshal produces it.
	server := Server{
		Config: "kube-apiserver-arg:\n- audit-log-path=/var/log/audit.log\n- audit-log-maxage=30\n",
	}

	if err := server.Validate(context.Background()); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}

	server.WithOidc(schemas.OidcConfig{
		Audience: types.StringValue("k3s-audience"),
		Issuer:   types.StringValue("https://oidc.example.com"),
	})

	args, ok := server.config["kube-apiserver-arg"].([]string)
	if !ok {
		t.Fatalf("kube-apiserver-arg is not a []string: %#v", server.config["kube-apiserver-arg"])
	}

	wantArgs := []string{
		"audit-log-path=/var/log/audit.log",
		"audit-log-maxage=30",
		"api-audiences=k3s-audience",
		"service-account-issuer=https://oidc.example.com",
		"service-account-issuer=k3s",
	}
	for _, want := range wantArgs {
		found := false
		for _, arg := range args {
			if arg == want {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("kube-apiserver-arg = %#v, missing %q", args, want)
		}
	}
	if len(args) != 8 {
		t.Errorf("kube-apiserver-arg has %d entries, want 8: %#v", len(args), args)
	}
}

func TestServerWithOidcSingleStringApiServerArg(t *testing.T) {
	// k3s accepts a single string for list-valued keys.
	server := Server{
		Config: "kube-apiserver-arg: audit-log-path=/var/log/audit.log\n",
	}

	if err := server.Validate(context.Background()); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}

	server.WithOidc(schemas.OidcConfig{
		Audience: types.StringValue("k3s-audience"),
		Issuer:   types.StringValue("https://oidc.example.com"),
	})

	args, ok := server.config["kube-apiserver-arg"].([]string)
	if !ok {
		t.Fatalf("kube-apiserver-arg is not a []string: %#v", server.config["kube-apiserver-arg"])
	}
	if len(args) != 7 {
		t.Fatalf("kube-apiserver-arg has %d entries, want 7: %#v", len(args), args)
	}
	if args[0] != "audit-log-path=/var/log/audit.log" {
		t.Errorf("kube-apiserver-arg[0] = %q, want original arg preserved", args[0])
	}
}

func TestServerWithOidcNoExistingApiServerArgs(t *testing.T) {
	server := Server{}

	if err := server.Validate(context.Background()); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}

	server.WithOidc(schemas.OidcConfig{
		Audience: types.StringValue("k3s-audience"),
		Issuer:   types.StringValue("https://oidc.example.com"),
	})

	args, ok := server.config["kube-apiserver-arg"].([]string)
	if !ok {
		t.Fatalf("kube-apiserver-arg is not a []string: %#v", server.config["kube-apiserver-arg"])
	}
	if len(args) != 6 {
		t.Errorf("kube-apiserver-arg has %d entries, want 6: %#v", len(args), args)
	}
}

func TestServerDataDir(t *testing.T) {
	server := Server{
		Config: "data-dir: /opt/rancher/k3s\n",
	}

	if err := server.Validate(context.Background()); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}

	if got, want := server.dataDir(), "/opt/rancher/k3s"; got != want {
		t.Errorf("dataDir() = %q, want %q", got, want)
	}

	defaulted := Server{}
	if err := defaulted.Validate(context.Background()); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
	if got, want := defaulted.dataDir(), DATA_DIR; got != want {
		t.Errorf("dataDir() = %q, want %q", got, want)
	}
}

func TestServerInstallCommandQuotesValues(t *testing.T) {
	server := Server{
		Token:   "tok'en with space",
		Version: "v1.32.6+k3s1",
		Env: map[string]string{
			"HTTP_PROXY": "http://user:pa'ss word@proxy:3128",
		},
	}

	if err := server.Validate(context.Background()); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}

	command := server.installCommand()
	wantParts := []string{
		`K3S_TOKEN='tok'\''en with space'`,
		"INSTALL_K3S_VERSION='v1.32.6+k3s1'",
		`HTTP_PROXY='http://user:pa'\''ss word@proxy:3128'`,
		"INSTALL_K3S_BIN_DIR='/usr/local/bin'",
	}
	for _, want := range wantParts {
		if !strings.Contains(command, want) {
			t.Errorf("installCommand() = %q, want %q", command, want)
		}
	}
}

func TestParseK3sVersionOutput(t *testing.T) {
	output := `k3s version v1.32.6+k3s1 (eb603acd)
go version go1.23.10
`

	version, err := parseK3sVersionOutput(output)
	if err != nil {
		t.Fatalf("parseK3sVersionOutput() error = %v", err)
	}

	if got, want := version, "v1.32.6+k3s1"; got != want {
		t.Errorf("parseK3sVersionOutput() = %q, want %q", got, want)
	}
}

func TestParseK3sVersionOutputError(t *testing.T) {
	if _, err := parseK3sVersionOutput("go version go1.23.10"); err == nil {
		t.Fatalf("parseK3sVersionOutput() expected error")
	}
}
