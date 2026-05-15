package schemas_test

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
	"striveworks.us/terraform-provider-k3s/internal/schemas"
)

func TestHaConfig_Validate(t *testing.T) {
	tests := []struct {
		name        string
		config      schemas.HaConfig
		expectError bool
	}{
		{
			name: "ClusterInit false, missing token",
			config: schemas.HaConfig{
				ClusterInit: types.BoolValue(false),
				Token:       types.StringNull(),
				Server:      types.StringValue("some-server"),
			},
			expectError: true,
		},
		{
			name: "ClusterInit false, missing server",
			config: schemas.HaConfig{
				ClusterInit: types.BoolValue(false),
				Token:       types.StringValue("some-token"),
				Server:      types.StringNull(),
			},
			expectError: true,
		},
		{
			name: "ClusterInit false, token and server provided",
			config: schemas.HaConfig{
				ClusterInit: types.BoolValue(false),
				Token:       types.StringValue("some-token"),
				Server:      types.StringValue("some-server"),
			},
			expectError: false,
		},
		{
			name: "ClusterInit true, token provided",
			config: schemas.HaConfig{
				ClusterInit: types.BoolValue(true),
				Token:       types.StringValue("some-token"),
				Server:      types.StringNull(),
			},
			expectError: true,
		},
		{
			name: "ClusterInit true, server provided",
			config: schemas.HaConfig{
				ClusterInit: types.BoolValue(true),
				Token:       types.StringNull(),
				Server:      types.StringValue("some-server"),
			},
			expectError: true,
		},
		{
			name: "ClusterInit true, token and server null",
			config: schemas.HaConfig{
				ClusterInit: types.BoolValue(true),
				Token:       types.StringNull(),
				Server:      types.StringNull(),
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
