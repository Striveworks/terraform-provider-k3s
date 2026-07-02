package provider

import (
	"context"
	"testing"

	frameworkresource "github.com/hashicorp/terraform-plugin-framework/resource"
	resourceschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
)

func TestK3sKubeConfigResourceMetadata(t *testing.T) {
	r := NewK3sKubeConfigResource()
	resp := &frameworkresource.MetadataResponse{}

	r.Metadata(context.Background(), frameworkresource.MetadataRequest{ProviderTypeName: "k3s"}, resp)

	if got, want := resp.TypeName, "k3s_kubeconfig"; got != want {
		t.Errorf("TypeName = %q, want %q", got, want)
	}
}

func TestK3sKubeConfigResourceAuthHostDoesNotRequireReplace(t *testing.T) {
	authSchema, ok := kubeConfigResourceSSHSchema().(resourceschema.SingleNestedAttribute)
	if !ok {
		t.Fatalf("kubeConfigResourceSSHSchema() = %T, want resourceschema.SingleNestedAttribute", kubeConfigResourceSSHSchema())
	}

	hostSchema, ok := authSchema.Attributes["host"].(resourceschema.StringAttribute)
	if !ok {
		t.Fatalf("auth.host schema = %T, want resourceschema.StringAttribute", authSchema.Attributes["host"])
	}

	if got := len(hostSchema.PlanModifiers); got != 0 {
		t.Fatalf("auth.host has %d plan modifiers, want 0", got)
	}
}
