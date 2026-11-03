package ssh_client

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"

	tftypes "github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/hashicorp/terraform-plugin-log/tfsdklog"
	"golang.org/x/crypto/ssh"
)

// tflog stores masking configuration and logger fields in the context and
// mutates them on every log call, so logging the two output pipes from
// separate goroutines used to crash the provider with "concurrent map
// iteration and map write". Run with -race to catch a regression.
func TestSSHClient_streamLogsIsRaceFree(t *testing.T) {
	ctx := tfsdklog.NewRootProviderLogger(context.Background(), tfsdklog.WithLevel(0))
	ctx = tflog.SetField(ctx, "tf_rpc", "ApplyResourceChange")
	ctx = tflog.SetField(ctx, "tf_resource_type", "k3s_agent")
	ctx = tflog.MaskLogStrings(ctx, "super-secret-password")

	client := SSHClient{ctx: ctx}

	const lineCount = 500
	stdout := strings.NewReader(repeatLines("stdout line", lineCount))
	stderr := strings.NewReader(repeatLines("stderr line", lineCount))

	done := make(chan struct{})
	go func() {
		defer close(done)
		client.streamLogs(stdout, stderr)
	}()

	select {
	case <-done:
	case <-time.After(30 * time.Second):
		t.Fatal("streamLogs did not return; output pipes are likely deadlocked")
	}
}

// A failing command reports its stderr, capped to the most recent lines, so
// the reason does not stay buried in debug-level logs.
func TestSSHClient_streamLogsReturnsStderrTail(t *testing.T) {
	ctx := tfsdklog.NewRootProviderLogger(context.Background(), tfsdklog.WithLevel(0))
	client := SSHClient{ctx: ctx}

	stdout := strings.NewReader(repeatLines("stdout line", 5))
	stderr := strings.NewReader(repeatLines("stderr line", stderrTailLines+5))

	tail := client.streamLogs(stdout, stderr)

	if len(tail) != stderrTailLines {
		t.Fatalf("tail length = %d, want %d", len(tail), stderrTailLines)
	}
	if want := fmt.Sprintf("stderr line %d", stderrTailLines+4); tail[len(tail)-1] != want {
		t.Errorf("last tail line = %q, want %q", tail[len(tail)-1], want)
	}
	if want := "stderr line 5"; tail[0] != want {
		t.Errorf("first tail line = %q, want %q", tail[0], want)
	}
	for _, line := range tail {
		if strings.Contains(line, "stdout") {
			t.Errorf("stdout leaked into stderr tail: %q", line)
		}
	}
}

// Both pipes can report a read error, which must not block the readers.
func TestSSHClient_streamLogsHandlesPipeErrors(t *testing.T) {
	ctx := tfsdklog.NewRootProviderLogger(context.Background(), tfsdklog.WithLevel(0))
	client := SSHClient{ctx: ctx}

	done := make(chan struct{})
	go func() {
		defer close(done)
		client.streamLogs(failingReader{}, failingReader{})
	}()

	select {
	case <-done:
	case <-time.After(30 * time.Second):
		t.Fatal("streamLogs did not return when both pipes failed")
	}
}

type failingReader struct{}

func (failingReader) Read([]byte) (int, error) {
	return 0, fmt.Errorf("pipe closed")
}

func repeatLines(prefix string, count int) string {
	var builder strings.Builder
	for i := range count {
		fmt.Fprintf(&builder, "%s %d\n", prefix, i)
	}
	return builder.String()
}

var _ io.Reader = failingReader{}

// Users supply host keys as text (authorized_keys format from ssh-keyscan, or
// a known_hosts line), not SSH binary wire format, so NewSSHClient must accept
// both text formats.
func TestNewSSHClient_hostKeyFormats(t *testing.T) {
	pub, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generating ed25519 key: %v", err)
	}
	sshPub, err := ssh.NewPublicKey(pub)
	if err != nil {
		t.Fatalf("converting to ssh public key: %v", err)
	}
	authorizedKey := string(ssh.MarshalAuthorizedKey(sshPub))

	cases := []struct {
		name    string
		hostKey string
		wantErr bool
	}{
		{name: "authorized_keys format", hostKey: authorizedKey},
		{name: "known_hosts format", hostKey: "example.com " + authorizedKey},
		{name: "garbage", hostKey: "not a valid key", wantErr: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			config := SSHConfig{
				User:    tftypes.StringValue("root"),
				Host:    tftypes.StringValue("example.com"),
				Port:    tftypes.Int32Value(22),
				HostKey: tftypes.StringValue(tc.hostKey),
			}

			client, err := NewSSHClient(context.Background(), config)
			if tc.wantErr {
				if err == nil {
					t.Fatal("NewSSHClient accepted an unparseable host key")
				}
				return
			}
			if err != nil {
				t.Fatalf("NewSSHClient: %v", err)
			}
			if client.Config.HostKeyCallback == nil {
				t.Fatal("expected a host key callback to be configured")
			}
			if err := client.Config.HostKeyCallback("example.com:22", nil, sshPub); err != nil {
				t.Errorf("host key callback rejected the configured key: %v", err)
			}
		})
	}
}

// A hung host must not block an apply for the OS TCP timeout.
func TestNewSSHClient_setsDialTimeout(t *testing.T) {
	config := SSHConfig{
		User: tftypes.StringValue("root"),
		Host: tftypes.StringValue("example.com"),
		Port: tftypes.Int32Value(22),
	}

	client, err := NewSSHClient(context.Background(), config)
	if err != nil {
		t.Fatalf("NewSSHClient: %v", err)
	}
	if client.Config.Timeout != 30*time.Second {
		t.Errorf("Config.Timeout = %v, want %v", client.Config.Timeout, 30*time.Second)
	}
}

// With missingOk, a missing file must produce truly empty output: callers
// compare the result against "" to detect absence, and the old fallback of
// echoing an empty string emitted a newline instead.
func TestReadFileCommand(t *testing.T) {
	const path = "/etc/rancher/k3s/config.yaml"

	cases := []struct {
		name      string
		missingOk bool
		sudo      bool
		want      string
	}{
		{
			name: "plain",
			want: "cat " + path,
		},
		{
			name: "sudo",
			sudo: true,
			want: "sudo cat " + path,
		},
		{
			name:      "missingOk",
			missingOk: true,
			want:      "sudo [ -f " + path + " ] && cat " + path + " || true",
		},
		{
			name:      "missingOk sudo",
			missingOk: true,
			sudo:      true,
			want:      "sudo [ -f " + path + " ] && sudo cat " + path + " || true",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := readFileCommand(path, tc.missingOk, tc.sudo); got != tc.want {
				t.Errorf("readFileCommand(%q, %v, %v) = %q, want %q", path, tc.missingOk, tc.sudo, got, tc.want)
			}
		})
	}
}
