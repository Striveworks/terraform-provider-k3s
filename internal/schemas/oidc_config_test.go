package schemas_test

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
	"striveworks.us/terraform-provider-k3s/internal/schemas"
)

func TestOidcConfig_Validate(t *testing.T) {
	tests := []struct {
		name        string
		config      schemas.OidcConfig
		expectError bool
	}{
		{
			name: "all fields provided",
			config: schemas.OidcConfig{
				Audience:     types.StringValue("k3s"),
				SigningPKCS8: types.StringValue("public-key"),
				SigningKey:   types.StringValue("private-key"),
				Issuer:       types.StringValue("https://issuer.example.com"),
			},
			expectError: false,
		},
		{
			name: "missing audience",
			config: schemas.OidcConfig{
				Audience:     types.StringNull(),
				SigningPKCS8: types.StringValue("public-key"),
				SigningKey:   types.StringValue("private-key"),
				Issuer:       types.StringValue("https://issuer.example.com"),
			},
			expectError: true,
		},
		{
			name: "missing signing fields and issuer",
			config: schemas.OidcConfig{
				Audience:     types.StringValue("k3s"),
				SigningPKCS8: types.StringNull(),
				SigningKey:   types.StringNull(),
				Issuer:       types.StringNull(),
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.expectError && err == nil {
				t.Fatalf("expected an error")
			}
			if !tt.expectError && err != nil {
				t.Fatalf("expected no error, got: %v", err)
			}
		})
	}
}
