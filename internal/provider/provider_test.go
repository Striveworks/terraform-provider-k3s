package provider

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/docker/docker/api/types/build"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/moby/go-archive"

	"striveworks.us/terraform-provider-k3s/internal/ssh_client"
)

// testAccProtoV6ProviderFactories is used to instantiate a provider during acceptance testing.
// The factory function is called for each Terraform CLI command to create a provider
// server that the CLI can connect to and interact with.
var testAccProtoV6ProviderFactories = map[string]func() (tfprotov6.ProviderServer, error){
	"k3s": providerserver.NewProtocol6WithError(NewDebugMode("")()),
}

type DockerComposeTestHarness struct {
	t            *testing.T
	Servers      map[string]*ServerInfo
	dockerClient *client.Client
	imageTag     string
	containerIDs []string
	dockerfile   string
}

type ServerInfo struct {
	Name        string
	Host        string
	ContainerIP string
	Port        int
	APIPort     int
	SSHClient   ssh_client.SSHClient
	ContainerID string
}

func NewDockerComposeTestHarness(t *testing.T, serverNames []string) (*DockerComposeTestHarness, error) {
	return NewDockerComposeTestHarnessWithDockerfile(t, serverNames, "Dockerfile")
}

func NewDockerComposeTestHarnessWithDockerfile(t *testing.T, serverNames []string, dockerfile string) (*DockerComposeTestHarness, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("failed to create docker client: %w", err)
	}

	buildContextPath, err := filepath.Abs("../../tests")
	if err != nil {
		return nil, fmt.Errorf("failed to get absolute path for build context: %w", err)
	}

	hash, err := getContextHash(buildContextPath, dockerfile)
	if err != nil {
		return nil, fmt.Errorf("failed to get build context hash: %w", err)
	}
	imageTag := fmt.Sprintf("tf-k3s-test-img-%s:%s", strings.ToLower(sanitizeDockerfileName(dockerfile)), hash)

	harness := &DockerComposeTestHarness{
		t:            t,
		Servers:      make(map[string]*ServerInfo),
		dockerClient: cli,
		imageTag:     imageTag,
		dockerfile:   dockerfile,
	}

	for _, name := range serverNames {
		harness.Servers[name] = &ServerInfo{Name: name, Host: "localhost"}
	}

	return harness, nil
}

func (h *DockerComposeTestHarness) Setup() error {
	ctx := context.Background()

	images, err := h.dockerClient.ImageList(ctx, image.ListOptions{
		Filters: filters.NewArgs(filters.Arg("reference", h.imageTag)),
	})
	if err != nil {
		return fmt.Errorf("failed to list docker images: %w", err)
	}

	if len(images) == 0 {
		h.t.Logf("Image %s not found, building it...", h.imageTag)

		buildContextPath, err := filepath.Abs("../../tests")
		if err != nil {
			return fmt.Errorf("failed to get absolute path for build context: %w", err)
		}

		tar, err := archive.TarWithOptions(buildContextPath, &archive.TarOptions{
			IncludeFiles: []string{h.dockerfile},
		})
		if err != nil {
			return fmt.Errorf("failed to create tar from build context: %w", err)
		}

		opts := build.ImageBuildOptions{
			Dockerfile: h.dockerfile,
			Tags:       []string{h.imageTag},
			Remove:     true,
		}
		res, err := h.dockerClient.ImageBuild(ctx, tar, opts)
		if err != nil {
			return fmt.Errorf("failed to build image: %w", err)
		}
		defer res.Body.Close()

		// We must read the response body to completion for the build to finish.
		// We discard it here to keep test logs clean.
		// NOTE: Building a docker image can take longer than the default 30s test
		// timeout. If this test fails due to a timeout, run it with a longer
		// timeout, e.g., `go test -timeout 5m`
		if _, err := io.Copy(io.Discard, res.Body); err != nil {
			return fmt.Errorf("failed to build docker: %w", err)
		}

		h.t.Log("Image built.")
	} else {
		h.t.Logf("Using existing image %s", h.imageTag)
	}

	for name, server := range h.Servers {
		h.t.Logf("Creating container for server %s...", name)

		containerConfig := &container.Config{
			Image:      h.imageTag,
			Tty:        false,
			StopSignal: "SIGRTMIN+3",
			ExposedPorts: nat.PortSet{
				"22/tcp":   struct{}{},
				"6443/tcp": struct{}{},
			},
		}

		hostConfig := &container.HostConfig{
			Privileged:   true,
			CgroupnsMode: container.CgroupnsModeHost,
			Tmpfs: map[string]string{
				"/tmp":      "",
				"/run":      "",
				"/run/lock": "",
			},
			Binds: []string{
				"/sys/fs/cgroup:/sys/fs/cgroup:rw",
			},
			PortBindings: nat.PortMap{
				"22/tcp": []nat.PortBinding{
					{
						HostIP:   "0.0.0.0",
						HostPort: "0", // Let docker assign a random port
					},
				},
				"6443/tcp": []nat.PortBinding{
					{
						HostIP:   "0.0.0.0",
						HostPort: "0", // Let docker assign a random port
					},
				},
			},
		}

		resp, err := h.dockerClient.ContainerCreate(ctx, containerConfig, hostConfig, nil, nil, name)
		if err != nil {
			return fmt.Errorf("failed to create container for server %s: %w", name, err)
		}
		server.ContainerID = resp.ID
		h.containerIDs = append(h.containerIDs, resp.ID)

		if err := h.dockerClient.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
			return fmt.Errorf("failed to start container for server %s: %w", name, err)
		}

		insp, err := h.dockerClient.ContainerInspect(ctx, resp.ID)
		if err != nil {
			return fmt.Errorf("failed to inspect container for server %s: %w", name, err)
		}

		port, err := publishedPort(insp.NetworkSettings.Ports, "22/tcp")
		if err != nil {
			return fmt.Errorf("failed to get ssh port for server %s: %w", name, err)
		}
		server.Port = port

		apiPort, err := publishedPort(insp.NetworkSettings.Ports, "6443/tcp")
		if err != nil {
			return fmt.Errorf("failed to get api port for server %s: %w", name, err)
		}
		server.APIPort = apiPort

		containerIP, err := containerIPAddress(insp.NetworkSettings)
		if err != nil {
			return fmt.Errorf("failed to get container ip for server %s: %w", name, err)
		}
		server.ContainerIP = containerIP

		h.t.Logf("Server %s is on SSH port %d, API port %d, and container IP %s", name, server.Port, server.APIPort, server.ContainerIP)

		var sshClient ssh_client.SSHClient
		h.t.Logf("Waiting for SSH to be available on port %d for server %s...", server.Port, name)

		sshConfig := ssh_client.SSHConfig{
			Host:     types.StringValue(server.Host),
			Port:     types.Int32Value(int32(server.Port)),
			User:     types.StringValue("root"),
			Password: types.StringValue("rootpassword"),
		}

		var ready bool
		var lastErr error
		for i := 0; i < 30; i++ {
			sshClient, err = ssh_client.NewSSHClient(context.Background(), sshConfig)
			if err != nil {
				lastErr = err
			} else if _, err := sshClient.Run("whoami"); err != nil {
				lastErr = err
			} else {
				h.t.Logf("SSH is ready for server %s", name)
				ready = true
				break
			}
			time.Sleep(2 * time.Second)
		}
		if !ready {
			return fmt.Errorf("SSH did not become ready for server %s: %w", name, lastErr)
		}
		server.SSHClient = sshClient
	}

	return nil
}

func (h *DockerComposeTestHarness) Teardown() {
	ctx := context.Background()
	h.t.Log("Tearing down docker environment...")
	for _, id := range h.containerIDs {
		h.t.Logf("Stopping and removing container %s...", id[:12])
		if err := h.dockerClient.ContainerStop(ctx, id, container.StopOptions{}); err != nil {
			h.t.Logf("Failed to stop container %s: %s", id, err)
		}
		if err := h.dockerClient.ContainerRemove(ctx, id, container.RemoveOptions{Force: true}); err != nil {
			h.t.Logf("Failed to remove container %s: %s", id, err)
		}
	}

	h.dockerClient.Close()
	h.t.Log("Docker environment torn down.")
}

func (h *DockerComposeTestHarness) GetServer(name string) (*ServerInfo, error) {
	server, ok := h.Servers[name]
	if !ok {
		return nil, fmt.Errorf("server %s not found in test harness", name)
	}
	return server, nil
}

func TestDockerHarness(t *testing.T) {
	skipUnlessAcc(t)

	serverNames := []string{"server1", "server2"}
	h, err := NewDockerComposeTestHarness(t, serverNames)
	if err != nil {
		t.Fatalf("Failed to create test harness: %s", err)
	}
	defer h.Teardown()

	if err := h.Setup(); err != nil {
		t.Fatalf("Failed to set up test environment: %s", err)
	}

	for _, name := range serverNames {
		server, err := h.GetServer(name)
		if err != nil {
			t.Fatal(err)
		}

		res, err := server.SSHClient.Run("ls -l /")
		if err != nil {
			t.Fatalf("Failed to run command on server %s: %s", name, err)
		}

		if len(res) == 0 {
			t.Fatalf("Expected output from command on server %s, but got none", name)
		}
		t.Logf("Successfully ran command on server %s", name)
	}
}

func getContextHash(path string, dockerfile string) (string, error) {
	dockerfilePath := filepath.Join(path, dockerfile)
	f, err := os.Open(dockerfilePath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}

	return fmt.Sprintf("%x", h.Sum(nil))[:12], nil
}

func sanitizeDockerfileName(name string) string {
	cleaned := make([]rune, 0, len(name))
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			cleaned = append(cleaned, r)
			continue
		}
		cleaned = append(cleaned, '-')
	}
	return string(cleaned)
}

func k3sAcceptanceEnvBlock(dockerfile string) string {
	if dockerfile != "Dockerfile.rocky10" {
		return ""
	}

	return `
	env = {
		INSTALL_K3S_SKIP_SELINUX_RPM = "true"
		INSTALL_K3S_SELINUX_WARN     = "true"
	}
`
}

func publishedPort(ports nat.PortMap, port nat.Port) (int, error) {
	portBindings, ok := ports[port]
	if !ok || len(portBindings) == 0 {
		return 0, fmt.Errorf("no port bindings found for %s", port)
	}

	published, err := strconv.Atoi(portBindings[0].HostPort)
	if err != nil {
		return 0, fmt.Errorf("failed to parse host port for %s: %w", port, err)
	}

	return published, nil
}

func containerIPAddress(settings *container.NetworkSettings) (string, error) {
	if settings == nil {
		return "", fmt.Errorf("network settings are nil")
	}

	if network := settings.Networks["bridge"]; network != nil && network.IPAddress != "" {
		return network.IPAddress, nil
	}

	networkNames := make([]string, 0, len(settings.Networks))
	for name := range settings.Networks {
		networkNames = append(networkNames, name)
	}
	sort.Strings(networkNames)

	for _, name := range networkNames {
		network := settings.Networks[name]
		if network != nil && network.IPAddress != "" {
			return network.IPAddress, nil
		}
	}

	return "", fmt.Errorf("no container IP address found")
}

func skipUnlessAcc(t *testing.T) {
	t.Helper()

	if os.Getenv("TF_ACC") == "" {
		t.Skip("set TF_ACC=1 to run Docker-backed acceptance tests")
	}
}

func waitForLocalAPIPort(host string, port int, timeout time.Duration) error {
	address := net.JoinHostPort(host, fmt.Sprintf("%d", port))
	deadline := time.Now().Add(timeout)
	var lastErr error

	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", address, 2*time.Second)
		if err == nil {
			conn.Close()
			return nil
		}

		lastErr = err
		time.Sleep(2 * time.Second)
	}

	return fmt.Errorf("timed out waiting for k3s api port %s: %w", address, lastErr)
}

func waitForSSHCommand(server *ServerInfo, command string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	var lastErr error

	for time.Now().Before(deadline) {
		if _, err := server.SSHClient.Run(command); err == nil {
			return nil
		} else {
			lastErr = err
		}

		time.Sleep(5 * time.Second)
	}

	return fmt.Errorf("timed out waiting for command %q: %w", command, lastErr)
}
