package ssh_client

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
	"golang.org/x/crypto/ssh"
)

func testPublicKey(t *testing.T) ssh.PublicKey {
	t.Helper()

	pub, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generating host key: %v", err)
	}

	key, err := ssh.NewPublicKey(pub)
	if err != nil {
		t.Fatalf("wrapping host key: %v", err)
	}

	return key
}

func testSSHConfig() SSHConfig {
	return SSHConfig{
		User:     types.StringValue("root"),
		Host:     types.StringValue("example.com"),
		Port:     types.Int32Value(22),
		Password: types.StringValue("s3cr3t"),
	}
}

// Checks that the client pinned the expected host key and rejects any other.
func assertPinnedHostKey(t *testing.T, client SSHClient, expected ssh.PublicKey) {
	t.Helper()

	addr := &net.TCPAddr{IP: net.ParseIP("192.0.2.10"), Port: 22}
	if err := client.Config.HostKeyCallback("example.com:22", addr, expected); err != nil {
		t.Errorf("expected host key to be accepted, got: %v", err)
	}
	if err := client.Config.HostKeyCallback("example.com:22", addr, testPublicKey(t)); err == nil {
		t.Errorf("expected a different host key to be rejected")
	}
}

func TestNewSSHClientHostKeyFormats(t *testing.T) {
	key := testPublicKey(t)
	authorized := string(ssh.MarshalAuthorizedKey(key))

	tests := []struct {
		name    string
		hostKey string
	}{
		{
			// An OpenSSH `*.pub` file, as found in /etc/ssh.
			name:    "Authorized key format",
			hostKey: authorized,
		},
		{
			// `ssh-keyscan` output or a known_hosts entry.
			name:    "Known hosts format",
			hostKey: fmt.Sprintf("example.com %s", authorized),
		},
		{
			// The RFC 4253 wire encoding, still accepted.
			name:    "Wire format",
			hostKey: string(key.Marshal()),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := testSSHConfig()
			config.HostKey = types.StringValue(tt.hostKey)

			client, err := NewSSHClient(context.Background(), config)
			if err != nil {
				t.Fatalf("expected host key to be accepted, got: %v", err)
			}

			assertPinnedHostKey(t, client, key)
		})
	}
}

func TestNewSSHClientHostKeyFile(t *testing.T) {
	key := testPublicKey(t)
	path := filepath.Join(t.TempDir(), "known_host.pub")
	if err := os.WriteFile(path, ssh.MarshalAuthorizedKey(key), 0o600); err != nil {
		t.Fatalf("writing host key file: %v", err)
	}

	config := testSSHConfig()
	config.HostKeyFile = types.StringValue(path)

	client, err := NewSSHClient(context.Background(), config)
	if err != nil {
		t.Fatalf("expected host key file to be accepted, got: %v", err)
	}

	assertPinnedHostKey(t, client, key)
}

func TestNewSSHClientInvalidHostKey(t *testing.T) {
	config := testSSHConfig()
	config.HostKey = types.StringValue("not-a-host-key")

	if _, err := NewSSHClient(context.Background(), config); err == nil {
		t.Fatalf("expected an invalid host key to be rejected")
	}
}

func TestNewSSHClientMissingHostKeyFile(t *testing.T) {
	config := testSSHConfig()
	config.HostKeyFile = types.StringValue(filepath.Join(t.TempDir(), "absent.pub"))

	if _, err := NewSSHClient(context.Background(), config); err == nil {
		t.Fatalf("expected a missing host key file to be rejected")
	}
}

func TestNewSSHClientWithoutHostKey(t *testing.T) {
	client, err := NewSSHClient(context.Background(), testSSHConfig())
	if err != nil {
		t.Fatalf("unexpected error building client: %v", err)
	}

	addr := &net.TCPAddr{IP: net.ParseIP("192.0.2.10"), Port: 22}
	if err := client.Config.HostKeyCallback("example.com:22", addr, testPublicKey(t)); err != nil {
		t.Errorf("expected host key verification to be skipped, got: %v", err)
	}
}
