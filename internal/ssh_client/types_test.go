package ssh_client

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
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
