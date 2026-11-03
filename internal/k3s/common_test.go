package k3s

import (
	"context"
	"strings"
	"testing"
)

func TestRegistryCommandsEmpty(t *testing.T) {
	// Validate always allocates a non-nil empty map for an empty registry
	// config; no commands may be produced for it, or an existing
	// host-managed registries.yaml would be clobbered with "{}".
	server := Server{}
	if err := server.Validate(context.Background()); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}

	commands, err := registryCommands(context.Background(), server.registry)
	if err != nil {
		t.Fatalf("registryCommands() error = %v", err)
	}
	if len(commands) != 0 {
		t.Errorf("registryCommands() = %#v, want no commands for empty registry", commands)
	}

	commands, err = registryCommands(context.Background(), nil)
	if err != nil {
		t.Fatalf("registryCommands() error = %v", err)
	}
	if len(commands) != 0 {
		t.Errorf("registryCommands() = %#v, want no commands for nil registry", commands)
	}
}

func TestRegistryCommandsWithRegistry(t *testing.T) {
	server := Server{
		Registry: "mirrors:\n  docker.io:\n    endpoint:\n    - https://registry.example.com\n",
	}
	if err := server.Validate(context.Background()); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}

	commands, err := registryCommands(context.Background(), server.registry)
	if err != nil {
		t.Fatalf("registryCommands() error = %v", err)
	}
	if len(commands) != 3 {
		t.Fatalf("registryCommands() produced %d commands, want 3: %#v", len(commands), commands)
	}
	if !strings.Contains(commands[1], "/etc/rancher/k3s/registries.yaml") {
		t.Errorf("registryCommands()[1] = %q, want write to registries.yaml", commands[1])
	}
}

func TestShellQuote(t *testing.T) {
	if got, want := shellQuote("token'with dollar$"), "'token'\\''with dollar$'"; got != want {
		t.Errorf("shellQuote() = %q, want %q", got, want)
	}
}
