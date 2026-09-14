package ssh_client

import (
	"context"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/hashicorp/terraform-plugin-log/tfsdklog"
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
