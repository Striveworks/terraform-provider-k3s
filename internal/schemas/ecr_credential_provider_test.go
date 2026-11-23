package schemas_test

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
	"striveworks.us/terraform-provider-k3s/internal/schemas"
)

func TestEcrCredentialProviderValidate(t *testing.T) {
	tests := []struct {
		name    string
		config  schemas.EcrCredentialProvider
		wantErr bool
	}{
		{
			name: "role only",
			config: schemas.EcrCredentialProvider{
				RoleARN: types.StringValue("arn:aws:iam::123456789012:role/ecr-reader"),
			},
		},
		{
			name: "missing role",
			config: schemas.EcrCredentialProvider{
				RoleARN: types.StringNull(),
			},
			wantErr: true,
		},
		{
			name: "empty role",
			config: schemas.EcrCredentialProvider{
				RoleARN: types.StringValue("  "),
			},
			wantErr: true,
		},
		{
			name: "bad url",
			config: schemas.EcrCredentialProvider{
				RoleARN: types.StringValue("arn:aws:iam::123456789012:role/ecr-reader"),
				URL:     types.StringValue("not a url"),
			},
			wantErr: true,
		},
		{
			name: "bad checksum",
			config: schemas.EcrCredentialProvider{
				RoleARN:  types.StringValue("arn:aws:iam::123456789012:role/ecr-reader"),
				Checksum: types.StringValue("abc"),
			},
			wantErr: true,
		},
		{
			name: "relative token file",
			config: schemas.EcrCredentialProvider{
				RoleARN:   types.StringValue("arn:aws:iam::123456789012:role/ecr-reader"),
				TokenFile: types.StringValue("ecr/token"),
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantErr && err == nil {
				t.Fatal("Validate() error = nil, want error")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("Validate() error = %v", err)
			}
		})
	}
}
