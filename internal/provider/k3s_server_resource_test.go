package provider

import (
	"fmt"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

func TestParseServerImportID(t *testing.T) {
	tests := map[string]struct {
		rawID          string
		user           string
		host           string
		port           int32
		password       string
		privateKeyFile string
		hostKeyFile    string
		binDir         string
	}{
		"password query": {
			rawID:    "ssh://root@example.com:2222?password=s3cr3t&bin_dir=/opt/bin",
			user:     "root",
			host:     "example.com",
			port:     2222,
			password: "s3cr3t",
			binDir:   "/opt/bin",
		},
		"password userinfo": {
			rawID:    "ssh://root:s3cr3t@example.com",
			user:     "root",
			host:     "example.com",
			port:     22,
			password: "s3cr3t",
			binDir:   "/usr/local/bin",
		},
		"private key file": {
			rawID:          "ssh://ubuntu@192.0.2.10?private_key_file=/home/me/.ssh/id_rsa&host_key_file=/home/me/.ssh/known_host.pub",
			user:           "ubuntu",
			host:           "192.0.2.10",
			port:           22,
			privateKeyFile: "/home/me/.ssh/id_rsa",
			hostKeyFile:    "/home/me/.ssh/known_host.pub",
			binDir:         "/usr/local/bin",
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			sshConfig, binDir, err := parseServerImportID(tt.rawID)
			if err != nil {
				t.Fatalf("parseServerImportID() error = %v", err)
			}

			if got := sshConfig.User.ValueString(); got != tt.user {
				t.Errorf("User = %q, want %q", got, tt.user)
			}
			if got := sshConfig.Host.ValueString(); got != tt.host {
				t.Errorf("Host = %q, want %q", got, tt.host)
			}
			if got := sshConfig.Port.ValueInt32(); got != tt.port {
				t.Errorf("Port = %d, want %d", got, tt.port)
			}
			if got := sshConfig.Password.ValueString(); got != tt.password {
				t.Errorf("Password = %q, want %q", got, tt.password)
			}
			if got := sshConfig.PrivateKeyFile.ValueString(); got != tt.privateKeyFile {
				t.Errorf("PrivateKeyFile = %q, want %q", got, tt.privateKeyFile)
			}
			if got := sshConfig.HostKeyFile.ValueString(); got != tt.hostKeyFile {
				t.Errorf("HostKeyFile = %q, want %q", got, tt.hostKeyFile)
			}
			if binDir != tt.binDir {
				t.Errorf("binDir = %q, want %q", binDir, tt.binDir)
			}
		})
	}
}

func TestParseServerImportIDError(t *testing.T) {
	tests := map[string]string{
		"missing scheme": "root@example.com",
		"missing user":   "ssh://example.com",
		"missing host":   "ssh://root@",
		"bad port":       "ssh://root@example.com:not-a-port?password=s3cr3t",
		"port zero":      "ssh://root@example.com:0?password=s3cr3t",
	}

	for name, rawID := range tests {
		t.Run(name, func(t *testing.T) {
			if _, _, err := parseServerImportID(rawID); err == nil {
				t.Fatalf("parseServerImportID() expected error")
			}
		})
	}
}

func TestAccK3sServerResource(t *testing.T) {
	runAccK3sServerResource(t, "Dockerfile")
}

func TestAccK3sServerResourceRocky10(t *testing.T) {
	runAccK3sServerResource(t, "Dockerfile.rocky10")
}

func runAccK3sServerResource(t *testing.T, dockerfile string) {
	skipUnlessAcc(t)

	prefix := fmt.Sprintf("server-%s-%d", sanitizeDockerfileName(dockerfile), time.Now().UnixNano())
	serverNames := []string{prefix + "-init"}
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
	envBlock := k3sAcceptanceEnvBlock(dockerfile)

	singleK3sServer := fmt.Sprintf(`
provider "k3s" {}

resource k3s_server "main" {
	auth = {
		user 	                     = "root",
		host 	                     = "localhost",
		password                     = "rootpassword",
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
	YAML
}

data "k3s_kubeconfig" "main" {
	auth = {
		user 	                     = "root",
		host 	                     = "localhost",
		password                     = "rootpassword",
		port                         = %d
	}

	hostname = "localhost"

	depends_on = [k3s_server.main]
}
`, server.Port, envBlock, server.Port)
	updatedK3sServer := fmt.Sprintf(`
provider "k3s" {}

resource k3s_server "main" {
	auth = {
		user 	                     = "root",
		host 	                     = "localhost",
		password                     = "rootpassword",
		port                         = %d
	}
%s

	config = <<-YAML
	disable-agent: true
	disable:
	  - traefik
	  - servicelb
	  - metrics-server
	write-kubeconfig-mode: "0644"
	tls-san:
	  - localhost
	YAML
}

data "k3s_kubeconfig" "main" {
	auth = {
		user 	                     = "root",
		host 	                     = "localhost",
		password                     = "rootpassword",
		port                         = %d
	}

	hostname = "localhost"

	depends_on = [k3s_server.main]
}
`, server.Port, envBlock, server.Port)
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: singleK3sServer,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("k3s_server.main", "token"),
					resource.TestCheckResourceAttrSet("k3s_server.main", "kubeconfig"),
					resource.TestCheckResourceAttrSet("data.k3s_kubeconfig.main", "kubeconfig"),
					resource.TestCheckResourceAttr("data.k3s_kubeconfig.main", "k3s_url", "https://localhost:6443"),
					resource.TestCheckResourceAttr("data.k3s_kubeconfig.main", "cluster_auth.server", "https://localhost:6443"),
					checkK3sServerInstalled(server),
				),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectSensitiveValue(
						"k3s_server.main",
						tfjsonpath.New("token"),
					),
					statecheck.ExpectSensitiveValue(
						"k3s_server.main",
						tfjsonpath.New("kubeconfig"),
					),
				},
			},
			{
				Config: updatedK3sServer,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("k3s_server.main", "token"),
					resource.TestCheckResourceAttrSet("k3s_server.main", "kubeconfig"),
					resource.TestCheckResourceAttrSet("data.k3s_kubeconfig.main", "kubeconfig"),
					resource.TestCheckResourceAttr("data.k3s_kubeconfig.main", "k3s_url", "https://localhost:6443"),
					resource.TestCheckResourceAttr("data.k3s_kubeconfig.main", "cluster_auth.server", "https://localhost:6443"),
					checkK3sServerInstalled(server),
					checkK3sServerUpdated(server),
				),
			},
		},
	})
}

func TestAccK3sServerResourceOrphan(t *testing.T) {
	skipUnlessAcc(t)

	prefix := fmt.Sprintf("server-orphan-%d", time.Now().UnixNano())
	serverNames := []string{prefix + "-init"}
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

	k3sServer := fmt.Sprintf(`
provider "k3s" {}

resource k3s_server "main" {
	auth = {
		user                         = "root"
		host                         = "localhost"
		password                     = "rootpassword"
		port                         = %d
	}

	orphan = true

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
	YAML
}
`, server.Port)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             checkK3sServerInstalled(server),
		Steps: []resource.TestStep{
			{
				Config: k3sServer,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("k3s_server.main", "orphan", "true"),
					checkK3sServerInstalled(server),
				),
			},
		},
	})
}

func TestAccK3sServerResourceHA(t *testing.T) {
	skipUnlessAcc(t)

	prefix := fmt.Sprintf("ha-%d", time.Now().UnixNano())
	serverNames := []string{prefix + "-init", prefix + "-join-1", prefix + "-join-2"}
	h, err := NewDockerComposeTestHarness(t, serverNames)
	if err != nil {
		t.Fatalf("Failed to create test harness: %s", err)
	}
	defer h.Teardown()

	if err := h.Setup(); err != nil {
		t.Fatalf("Failed to set up test environment: %s", err)
	}

	initServer, err := h.GetServer(serverNames[0])
	if err != nil {
		t.Fatal(err)
	}
	joinServer1, err := h.GetServer(serverNames[1])
	if err != nil {
		t.Fatal(err)
	}
	joinServer2, err := h.GetServer(serverNames[2])
	if err != nil {
		t.Fatal(err)
	}

	haK3sServers := fmt.Sprintf(`
provider "k3s" {}

resource k3s_server "init" {
	auth = {
		user 	                     = "root",
		host 	                     = "localhost",
		password                     = "rootpassword",
		port                         = %d
	}

	config = <<-YAML
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

	highly_available = {
		cluster_init = true
	}
}

resource k3s_server "join_1" {
	auth = {
		user 	                     = "root",
		host 	                     = "localhost",
		password                     = "rootpassword",
		port                         = %d
	}

	config = <<-YAML
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

	highly_available = {
		token  = k3s_server.init.token
		server = "https://%s:6443"
	}
}

resource k3s_server "join_2" {
	auth = {
		user 	                     = "root",
		host 	                     = "localhost",
		password                     = "rootpassword",
		port                         = %d
	}

	config = <<-YAML
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

	highly_available = {
		token  = k3s_server.init.token
		server = "https://%s:6443"
	}

	depends_on = [k3s_server.join_1]
}
`, initServer.Port, initServer.ContainerIP, joinServer1.Port, joinServer1.ContainerIP, initServer.ContainerIP, joinServer2.Port, joinServer2.ContainerIP, initServer.ContainerIP)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: haK3sServers,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("k3s_server.init", "active", "true"),
					resource.TestCheckResourceAttr("k3s_server.join_1", "active", "true"),
					resource.TestCheckResourceAttr("k3s_server.join_2", "active", "true"),
					resource.TestCheckResourceAttrSet("k3s_server.init", "token"),
					resource.TestCheckResourceAttrSet("k3s_server.join_1", "token"),
					resource.TestCheckResourceAttrSet("k3s_server.join_2", "token"),
					resource.TestCheckResourceAttrSet("k3s_server.init", "kubeconfig"),
					resource.TestCheckResourceAttrSet("k3s_server.join_1", "kubeconfig"),
					resource.TestCheckResourceAttrSet("k3s_server.join_2", "kubeconfig"),
					checkK3sHAServerInstalled(initServer, true, ""),
					checkK3sHAServerInstalled(joinServer1, false, initServer.ContainerIP),
					checkK3sHAServerInstalled(joinServer2, false, initServer.ContainerIP),
					checkK3sHAClusterReady(initServer, 3),
				),
			},
		},
	})
}

func checkK3sServerInstalled(server *ServerInfo) resource.TestCheckFunc {
	return func(_ *terraform.State) error {
		if err := waitForLocalAPIPort(server.Host, server.APIPort, 2*time.Minute); err != nil {
			return err
		}

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
				command: "sudo grep -q 'disable-agent: true' /etc/rancher/k3s/config.yaml",
			},
			{
				name:    "systemd service was generated",
				command: "sudo test -f /etc/systemd/system/k3s.service",
			},
			{
				name:    "k3s service is active",
				command: "sudo systemctl is-active --quiet k3s",
			},
			{
				name:    "server token was generated",
				command: "sudo test -s /var/lib/rancher/k3s/server/token || sudo grep -q '^K3S_TOKEN=' /etc/systemd/system/k3s.service.env",
			},
			{
				name:    "kubeconfig was generated",
				command: "sudo test -s /etc/rancher/k3s/k3s.yaml",
			},
		}

		for _, check := range checks {
			if _, err := server.SSHClient.Run(check.command); err != nil {
				return fmt.Errorf("%s: %w", check.name, err)
			}
		}

		return waitForSSHCommand(server, "sudo /usr/local/bin/k3s kubectl get --raw=/readyz", 2*time.Minute)
	}
}

func checkK3sHAServerInstalled(server *ServerInfo, clusterInit bool, initServerIP string) resource.TestCheckFunc {
	return func(_ *terraform.State) error {
		if err := waitForLocalAPIPort(server.Host, server.APIPort, 4*time.Minute); err != nil {
			return err
		}

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
				name:    "systemd service was generated",
				command: "sudo test -f /etc/systemd/system/k3s.service",
			},
			{
				name:    "k3s service is active",
				command: "sudo systemctl is-active --quiet k3s",
			},
			{
				name:    "server token was generated",
				command: "sudo test -s /var/lib/rancher/k3s/server/token",
			},
			{
				name:    "kubeconfig was generated",
				command: "sudo test -s /etc/rancher/k3s/k3s.yaml",
			},
			{
				name:    "ha cluster-init config was written",
				command: fmt.Sprintf("sudo grep -Fq 'cluster-init: %t' /etc/rancher/k3s/config.yaml", clusterInit),
			},
		}

		if !clusterInit {
			checks = append(checks, struct {
				name    string
				command string
			}{
				name:    "ha server config was written",
				command: fmt.Sprintf("sudo grep -Fq 'server: https://%s:6443' /etc/rancher/k3s/config.yaml", initServerIP),
			})
		}

		for _, check := range checks {
			if _, err := server.SSHClient.Run(check.command); err != nil {
				return fmt.Errorf("%s: %w", check.name, err)
			}
		}

		return waitForSSHCommand(server, "sudo /usr/local/bin/k3s kubectl get --raw=/readyz", 4*time.Minute)
	}
}

func checkK3sHAClusterReady(server *ServerInfo, nodeCount int) resource.TestCheckFunc {
	return func(_ *terraform.State) error {
		if err := waitForSSHCommand(server, "sudo /usr/local/bin/k3s kubectl wait --for=condition=Ready nodes --all --timeout=180s", 5*time.Minute); err != nil {
			return err
		}

		command := fmt.Sprintf("test \"$(sudo /usr/local/bin/k3s kubectl get nodes --no-headers | wc -l | tr -d ' ')\" = \"%d\"", nodeCount)
		if err := waitForSSHCommand(server, command, 2*time.Minute); err != nil {
			return fmt.Errorf("waiting for %d ready k3s nodes: %w", nodeCount, err)
		}

		return nil
	}
}

func checkK3sServerUpdated(server *ServerInfo) resource.TestCheckFunc {
	return func(_ *terraform.State) error {
		_, err := server.SSHClient.Run("sudo grep -q 'write-kubeconfig-mode: \"0644\"' /etc/rancher/k3s/config.yaml")
		if err != nil {
			return fmt.Errorf("updated config file was not written: %w", err)
		}

		return waitForSSHCommand(server, "sudo /usr/local/bin/k3s kubectl get --raw=/readyz", 2*time.Minute)
	}
}
