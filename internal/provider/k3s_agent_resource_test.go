package provider

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

func TestAccK3sAgentResource(t *testing.T) {
	runAccK3sAgentResource(t, "Dockerfile")
}

func TestAccK3sAgentResourceRocky10(t *testing.T) {
	runAccK3sAgentResource(t, "Dockerfile.rocky10")
}

func runAccK3sAgentResource(t *testing.T, dockerfile string) {
	skipUnlessAcc(t)

	prefix := fmt.Sprintf("agent-%d", time.Now().UnixNano())
	serverNames := []string{prefix + "-server", prefix + "-worker"}
	h, err := NewDockerComposeTestHarnessWithDockerfile(t, serverNames, dockerfile)
	if err != nil {
		t.Fatalf("Failed to create test harness: %s", err)
	}
	defer h.Teardown()

	if err := h.Setup(); err != nil {
		t.Fatalf("Failed to set up test environment: %s", err)
	}

	server, err := h.GetServer(serverNames[0])
	if err != nil {
		t.Fatal(err)
	}
	agent, err := h.GetServer(serverNames[1])
	if err != nil {
		t.Fatal(err)
	}
	envBlock := k3sAcceptanceEnvBlock(dockerfile)

	k3sAgent := fmt.Sprintf(`
provider "k3s" {}

resource k3s_server "main" {
	auth = {
		user                         = "root"
		host                         = "localhost"
		password                     = "rootpassword"
		port                         = %d
	}
%s

	config = <<-YAML
	disable-agent: true
	disable:
	  - traefik
	  - servicelb
	  - metrics-server
	snapshotter: native
	write-kubeconfig-mode: "0600"
	tls-san:
	  - localhost
	  - %s
	YAML
}

resource k3s_agent "main" {
	auth = {
		user                         = "root"
		host                         = "localhost"
		password                     = "rootpassword"
		port                         = %d
	}
%s

	server = "https://%s:6443"
	token  = k3s_server.main.token

	config = <<-YAML
	snapshotter: native
	node-label:
	  - acc-role=agent
	YAML
}
`, server.Port, envBlock, server.ContainerIP, agent.Port, envBlock, server.ContainerIP)

	updatedK3sAgent := fmt.Sprintf(`
provider "k3s" {}

resource k3s_server "main" {
	auth = {
		user                         = "root"
		host                         = "localhost"
		password                     = "rootpassword"
		port                         = %d
	}
%s

	config = <<-YAML
	disable-agent: true
	disable:
	  - traefik
	  - servicelb
	  - metrics-server
	snapshotter: native
	write-kubeconfig-mode: "0600"
	tls-san:
	  - localhost
	  - %s
	YAML
}

resource k3s_agent "main" {
	auth = {
		user                         = "root"
		host                         = "localhost"
		password                     = "rootpassword"
		port                         = %d
	}
%s

	server = "https://%s:6443"
	token  = k3s_server.main.token

	config = <<-YAML
	snapshotter: native
	node-label:
	  - acc-role=agent-updated
	YAML
}
`, server.Port, envBlock, server.ContainerIP, agent.Port, envBlock, server.ContainerIP)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: k3sAgent,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("k3s_server.main", "active", "true"),
					resource.TestCheckResourceAttr("k3s_agent.main", "active", "true"),
					resource.TestCheckResourceAttr("k3s_agent.main", "server", fmt.Sprintf("https://%s:6443", server.ContainerIP)),
					resource.TestCheckResourceAttrSet("k3s_agent.main", "token"),
					resource.TestCheckResourceAttrSet("k3s_agent.main", "version"),
					checkK3sServerInstalled(server),
					checkK3sAgentInstalled(agent, "acc-role=agent"),
					checkK3sAgentJoined(server, agent),
				),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectSensitiveValue(
						"k3s_server.main",
						tfjsonpath.New("token"),
					),
					statecheck.ExpectSensitiveValue(
						"k3s_agent.main",
						tfjsonpath.New("token"),
					),
				},
			},
			{
				Config: updatedK3sAgent,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("k3s_server.main", "active", "true"),
					resource.TestCheckResourceAttr("k3s_agent.main", "active", "true"),
					resource.TestCheckResourceAttrSet("k3s_agent.main", "version"),
					checkK3sAgentInstalled(agent, "acc-role=agent-updated"),
					checkK3sAgentJoined(server, agent),
				),
			},
		},
	})
}

func TestAccK3sAgentResourceOrphan(t *testing.T) {
	skipUnlessAcc(t)

	prefix := fmt.Sprintf("agent-orphan-%d", time.Now().UnixNano())
	serverNames := []string{prefix + "-server", prefix + "-worker"}
	h, err := NewDockerComposeTestHarness(t, serverNames)
	if err != nil {
		t.Fatalf("Failed to create test harness: %s", err)
	}
	defer h.Teardown()

	if err := h.Setup(); err != nil {
		t.Fatalf("Failed to set up test environment: %s", err)
	}

	server, err := h.GetServer(serverNames[0])
	if err != nil {
		t.Fatal(err)
	}
	agent, err := h.GetServer(serverNames[1])
	if err != nil {
		t.Fatal(err)
	}

	k3sAgent := fmt.Sprintf(`
provider "k3s" {}

resource k3s_server "main" {
	auth = {
		user                         = "root"
		host                         = "localhost"
		password                     = "rootpassword"
		port                         = %d
	}

	config = <<-YAML
	disable-agent: true
	disable:
	  - traefik
	  - servicelb
	  - metrics-server
	snapshotter: native
	write-kubeconfig-mode: "0600"
	tls-san:
	  - localhost
	  - %s
	YAML
}

resource k3s_agent "main" {
	auth = {
		user                         = "root"
		host                         = "localhost"
		password                     = "rootpassword"
		port                         = %d
	}

	orphan = true
	server = "https://%s:6443"
	token  = k3s_server.main.token

	config = <<-YAML
	snapshotter: native
	node-label:
	  - acc-role=agent-orphan
	YAML
}
`, server.Port, server.ContainerIP, agent.Port, server.ContainerIP)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             checkK3sAgentInstalled(agent, "acc-role=agent-orphan"),
		Steps: []resource.TestStep{
			{
				Config: k3sAgent,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("k3s_agent.main", "orphan", "true"),
					checkK3sAgentInstalled(agent, "acc-role=agent-orphan"),
					checkK3sAgentJoined(server, agent),
				),
			},
		},
	})
}

func checkK3sAgentInstalled(agent *ServerInfo, label string) resource.TestCheckFunc {
	return func(_ *terraform.State) error {
		checks := []struct {
			name    string
			command string
		}{
			{
				name:    "install script was synced",
				command: "sudo test -f /usr/local/bin/k3s-install.sh",
			},
			{
				name:    "k3s binary is installed",
				command: "sudo test -x /usr/local/bin/k3s",
			},
			{
				name:    "config file was written",
				command: fmt.Sprintf("sudo grep -Fq '%s' /etc/rancher/k3s/config.yaml", label),
			},
			{
				name:    "systemd service was generated",
				command: "sudo test -f /etc/systemd/system/k3s-agent.service",
			},
			{
				name:    "k3s-agent service is active",
				command: "sudo systemctl is-active --quiet k3s-agent",
			},
			{
				name:    "agent server url was written",
				command: "sudo grep -q '^K3S_URL=' /etc/systemd/system/k3s-agent.service.env",
			},
			{
				name:    "agent token was written",
				command: "sudo grep -q '^K3S_TOKEN=' /etc/systemd/system/k3s-agent.service.env",
			},
		}

		for _, check := range checks {
			if _, err := agent.SSHClient.Run(check.command); err != nil {
				return fmt.Errorf("%s: %w", check.name, err)
			}
		}

		return nil
	}
}

func checkK3sAgentJoined(server *ServerInfo, agent *ServerInfo) resource.TestCheckFunc {
	return func(_ *terraform.State) error {
		hostname, err := agent.SSHClient.Hostname()
		if err != nil {
			return fmt.Errorf("reading agent hostname: %w", err)
		}
		hostname = strings.TrimSpace(hostname)

		if err := waitForSSHCommand(server, "sudo /usr/local/bin/k3s kubectl wait --for=condition=Ready nodes --all --timeout=180s", 5*time.Minute); err != nil {
			return err
		}

		nodeCommand := fmt.Sprintf("sudo /usr/local/bin/k3s kubectl get node %s", hostname)
		if err := waitForSSHCommand(server, nodeCommand, 2*time.Minute); err != nil {
			return fmt.Errorf("waiting for agent node %q: %w", hostname, err)
		}

		countCommand := `test "$(sudo /usr/local/bin/k3s kubectl get nodes --no-headers | wc -l | tr -d ' ')" = "1"`
		if err := waitForSSHCommand(server, countCommand, 2*time.Minute); err != nil {
			return fmt.Errorf("waiting for single joined agent node: %w", err)
		}

		return nil
	}
}
