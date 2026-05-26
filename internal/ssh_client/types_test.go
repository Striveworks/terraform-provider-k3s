package ssh_client

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
)

func TestSSHConfig_Validate(t *testing.T) {
	tests := []struct {
		name        string
		config      SSHConfig
		expectError bool
	}{
		{
			name: "No credentials provided",
			config: SSHConfig{
				User: types.StringValue("testuser"),
				Host: types.StringValue("127.0.0.1"),
				Port: types.Int32Value(22),
			},
			expectError: true,
		},
		{
			name: "Only password provided",
			config: SSHConfig{
				User:     types.StringValue("testuser"),
				Host:     types.StringValue("127.0.0.1"),
				Port:     types.Int32Value(22),
				Password: types.StringValue("testpassword"),
			},
			expectError: false,
		},
		{
			name: "Only private key provided",
			config: SSHConfig{
				User:       types.StringValue("testuser"),
				Host:       types.StringValue("127.0.0.1"),
				Port:       types.Int32Value(22),
				PrivateKey: types.StringValue("-----BEGIN RSA PRIVATE KEY-----..."),
			},
			expectError: false,
		},
		{
			name: "Unknown private key defers validation",
			config: SSHConfig{
				User:       types.StringValue("testuser"),
				Host:       types.StringValue("127.0.0.1"),
				Port:       types.Int32Value(22),
				PrivateKey: types.StringUnknown(),
			},
			expectError: false,
		},
		{
			name: "Empty string private key is not a credential",
			config: SSHConfig{
				User:       types.StringValue("testuser"),
				Host:       types.StringValue("127.0.0.1"),
				Port:       types.Int32Value(22),
				PrivateKey: types.StringValue(""),
			},
			expectError: true,
		},
		{
			name: "Only private key file provided",
			config: SSHConfig{
				User:           types.StringValue("testuser"),
				Host:           types.StringValue("127.0.0.1"),
				Port:           types.Int32Value(22),
				PrivateKeyFile: types.StringValue("/path/to/key"),
			},
			expectError: false,
		},
		{
			name: "Multiple credentials provided (password and private key)",
			config: SSHConfig{
				User:       types.StringValue("testuser"),
				Host:       types.StringValue("127.0.0.1"),
				Port:       types.Int32Value(22),
				Password:   types.StringValue("testpassword"),
				PrivateKey: types.StringValue("-----BEGIN RSA PRIVATE KEY-----..."),
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.expectError {
				if err == nil {
					t.Errorf("Expected an error for test case '%s', but got none", tt.name)
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error for test case '%s', but got: %v", tt.name, err)
				}
			}
		})
	}
}

func TestSSHConfig_ObjectAsPreservesPrivateKey(t *testing.T) {
	ctx := context.Background()
	auth, diags := types.ObjectValue(
		SSHConfig{}.AttributeTypes(),
		map[string]attr.Value{
			"user":             types.StringValue("testuser"),
			"host":             types.StringValue("127.0.0.1"),
			"port":             types.Int32Value(22),
			"private_key":      types.StringValue("-----BEGIN RSA PRIVATE KEY-----\ntest\n-----END RSA PRIVATE KEY-----"),
			"private_key_file": types.StringNull(),
			"password":         types.StringNull(),
			"host_key":         types.StringNull(),
			"host_key_file":    types.StringNull(),
		},
	)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics creating auth object: %v", diags)
	}

	var config SSHConfig
	diags = auth.As(ctx, &config, basetypes.ObjectAsOptions{})
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics decoding auth object: %v", diags)
	}

	if got := config.PrivateKey.ValueString(); got == "" {
		t.Fatalf("expected private key to be preserved")
	}
}
