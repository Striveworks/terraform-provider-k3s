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
	if !strings.Contains(command, "INSTALL_K3S_BIN_DIR=/usr/local/bin") {
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

func TestShellQuote(t *testing.T) {
	if got, want := shellQuote("token'with dollar$"), "'token'\\''with dollar$'"; got != want {
		t.Errorf("shellQuote() = %q, want %q", got, want)
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
