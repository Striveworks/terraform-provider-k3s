package provider

import (
	"context"
	"fmt"
	"testing"

	frameworkresource "github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

func TestGenerateK3sToken(t *testing.T) {
	for range 100 {
		token, err := GenerateK3sToken()
		if err != nil {
			t.Fatalf("GenerateK3sToken() error = %v", err)
		}

		if !IsValidK3sToken(token) {
			t.Fatalf("GenerateK3sToken() = %q, want token matching %s", token, k3sTokenPattern.String())
		}
	}
}

func TestIsValidK3sToken(t *testing.T) {
	tests := []struct {
		name  string
		token string
		want  bool
	}{
		{
			name:  "valid",
			token: "abcdef.0123456789abcdef",
			want:  true,
		},
		{
			name:  "uppercase is invalid",
			token: "ABCDEF.0123456789abcdef",
			want:  false,
		},
		{
			name:  "short id is invalid",
			token: "abcde.0123456789abcdef",
			want:  false,
		},
		{
			name:  "short secret is invalid",
			token: "abcdef.0123456789abcde",
			want:  false,
		},
		{
			name:  "missing separator is invalid",
			token: "abcdef0123456789abcdef",
			want:  false,
		},
		{
			name:  "hyphen is invalid",
			token: "abc-def.0123456789abcdef",
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsValidK3sToken(tt.token); got != tt.want {
				t.Errorf("IsValidK3sToken(%q) = %v, want %v", tt.token, got, tt.want)
			}
		})
	}
}

func TestK3sTokenID(t *testing.T) {
	if got, want := k3sTokenID("abcdef.0123456789abcdef"), "abcdef"; got != want {
		t.Errorf("k3sTokenID() = %q, want %q", got, want)
	}
}

func TestK3sTokenResourceMetadata(t *testing.T) {
	r := NewK3sTokenResource()
	resp := &frameworkresource.MetadataResponse{}

	r.Metadata(context.Background(), frameworkresource.MetadataRequest{ProviderTypeName: "k3s"}, resp)

	if got, want := resp.TypeName, "k3s_token"; got != want {
		t.Errorf("TypeName = %q, want %q", got, want)
	}
}

func TestK3sTokenPatternMatchesDocumentedFormat(t *testing.T) {
	if got, want := k3sTokenPattern.String(), `^[a-z0-9]{6}\.[a-z0-9]{16}$`; got != want {
		t.Errorf("k3sTokenPattern = %q, want %q", got, want)
	}
}

func TestK3sTokenResourceLifecycle(t *testing.T) {
	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `resource "k3s_token" "main" {}`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrWith("k3s_token.main", "id", func(value string) error {
						if len(value) != k3sTokenIDLength {
							return fmt.Errorf("id length = %d, want %d", len(value), k3sTokenIDLength)
						}
						return nil
					}),
					resource.TestCheckResourceAttrWith("k3s_token.main", "token", func(value string) error {
						if !IsValidK3sToken(value) {
							return fmt.Errorf("token %q does not match %s", value, k3sTokenPattern.String())
						}
						return nil
					}),
				),
			},
			{
				ResourceName:      "k3s_token.main",
				ImportState:       true,
				ImportStateIdFunc: k3sTokenImportStateID,
				ImportStateVerify: true,
			},
		},
	})
}

func k3sTokenImportStateID(state *terraform.State) (string, error) {
	resourceState, ok := state.RootModule().Resources["k3s_token.main"]
	if !ok {
		return "", fmt.Errorf("k3s_token.main not found in state")
	}

	token := resourceState.Primary.Attributes["token"]
	if !IsValidK3sToken(token) {
		return "", fmt.Errorf("state token %q does not match %s", token, k3sTokenPattern.String())
	}

	return token, nil
}
