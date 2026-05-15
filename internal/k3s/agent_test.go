package k3s

import (
	"context"
	"strings"
	"testing"
)

func TestAgentValidateDefaultsBinDir(t *testing.T) {
	agent := Agent{
		Config: "data_dir: /opt/rancher/k3s\n",
	}

	if err := agent.Validate(context.Background()); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}

	if got, want := agent.BinDir, BIN_DIR; got != want {
		t.Errorf("BinDir = %q, want %q", got, want)
	}
	if got, want := agent.dataDir(), "/opt/rancher/k3s"; got != want {
		t.Errorf("dataDir() = %q, want %q", got, want)
	}
}

func TestAgentInstallCommand(t *testing.T) {
	agent := Agent{
		Token:   "join-token",
		Server:  "https://10.0.0.1:6443",
		Version: "v1.32.6+k3s1",
		BinDir:  "/opt/bin",
		Env: map[string]string{
			"INSTALL_K3S_CHANNEL": "stable",
		},
	}

	command := agent.installCommand()
	wantParts := []string{
		"INSTALL_K3S_SKIP_START=true",
		"BIN_DIR=/opt/bin",
		"INSTALL_K3S_EXEC='agent --config /etc/rancher/k3s/config.yaml'",
		"K3S_URL=https://10.0.0.1:6443",
		"K3S_TOKEN=join-token",
		"INSTALL_K3S_VERSION=\"v1.32.6+k3s1\"",
		"INSTALL_K3S_CHANNEL=\"stable\"",
		"bash /opt/bin/k3s-install.sh",
	}

	for _, want := range wantParts {
		if !strings.Contains(command, want) {
			t.Errorf("installCommand() = %q, want %q", command, want)
		}
	}
}
