package schemas

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
)

const (
	DefaultECRCredentialProviderURL      = "https://github.com/dntosas/ecr-credential-provider/releases/download/v1.2.0/ecr-credential-provider-linux-amd64"
	DefaultECRCredentialProviderChecksum = "6b9c0b8ba9c4b2ab1b3eb093044c9b4500eb2916b6b9f792a8923d91789cd1a4"
	DefaultECRCredentialProviderToken    = "/var/lib/rancher/k3s/agent/ecr/token"
	DefaultECRCredenialIRSAImage         = "cgr.dev/chainguard/busybox:latest"
)

// EcrCredentialProvider installs the kubelet ECR credential provider on a k3s node.
type EcrCredentialProvider struct {
	RoleARN   types.String `tfsdk:"role_arn"`
	URL       types.String `tfsdk:"url"`
	Checksum  types.String `tfsdk:"checksum"`
	TokenFile types.String `tfsdk:"token_file"`
	Image     types.String `tfsdk:"image"`
}

func (e EcrCredentialProvider) Schema() schema.Attribute {
	return schema.SingleNestedAttribute{
		Optional: true,
		Description: "Install the ECR kubelet image credential provider. Downloads the binary to " +
			"/usr/local/libexec/ecr-credential-provider, writes /etc/rancher/k3s/credential-provider-config.yaml, " +
			"and adds the kubelet image-credential-provider arguments. Hosts with fapolicyd also receive an execution rule. " +
			"Servers also write ecr-helper.yaml to the k3s server manifests directory, where k3s deploys a DaemonSet that projects the web identity token onto each node.",
		Attributes: map[string]schema.Attribute{
			"role_arn": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "IAM role ARN passed to the credential provider as AWS_ROLE_ARN.",
			},
			"url": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString(DefaultECRCredentialProviderURL),
				MarkdownDescription: "URL of the ecr-credential-provider binary. Defaults to the linux/amd64 v1.2.0 release. Override url and checksum together for another architecture or version.",
			},
			"checksum": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString(DefaultECRCredentialProviderChecksum),
				MarkdownDescription: "sha256 checksum of the credential provider binary.",
			},
			"token_file": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString(DefaultECRCredentialProviderToken),
				MarkdownDescription: "Absolute path passed as AWS_WEB_IDENTITY_TOKEN_FILE. The server manifest DaemonSet writes this file on each node.",
			},
			"image": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString(DefaultECRCredenialIRSAImage),
				MarkdownDescription: "The image passing the token to the binary. The default image is a public harnded chainguard image.",
			},
		},
	}
}

func (e EcrCredentialProvider) ToObject(ctx context.Context) basetypes.ObjectValue {
	return ToObject(ctx, e)
}

func (e EcrCredentialProvider) AttributeTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"role_arn":   types.StringType,
		"url":        types.StringType,
		"checksum":   types.StringType,
		"token_file": types.StringType,
		"image":      types.StringType,
	}
}

func (e EcrCredentialProvider) Validate() error {
	if e.RoleARN.IsNull() || (!e.RoleARN.IsUnknown() && strings.TrimSpace(e.RoleARN.ValueString()) == "") {
		return fmt.Errorf("role_arn is required")
	}
	if value := configuredString(e.URL); value != "" {
		parsed, err := url.Parse(value)
		if err != nil || (parsed.Scheme != "https" && parsed.Scheme != "http") || parsed.Host == "" {
			return fmt.Errorf("url must be an http(s) URL")
		}
	}
	if value := configuredString(e.Checksum); value != "" {
		if len(value) != 64 || strings.Trim(value, "0123456789abcdefABCDEF") != "" {
			return fmt.Errorf("checksum must be a sha256 hex digest")
		}
	}
	if value := configuredString(e.TokenFile); value != "" && !strings.HasPrefix(value, "/") {
		return fmt.Errorf("token_file must be an absolute path")
	}
	return nil
}

func configuredString(value types.String) string {
	if value.IsNull() || value.IsUnknown() {
		return ""
	}
	return strings.TrimSpace(value.ValueString())
}
