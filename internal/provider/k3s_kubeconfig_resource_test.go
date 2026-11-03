package provider

import (
	"context"
	"testing"

	frameworkresource "github.com/hashicorp/terraform-plugin-framework/resource"
	resourceschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/defaults"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"striveworks.us/terraform-provider-k3s/internal/k3s"
)

func TestK3sKubeConfigResourceMetadata(t *testing.T) {
	r := NewK3sKubeConfigResource()
	resp := &frameworkresource.MetadataResponse{}

	r.Metadata(context.Background(), frameworkresource.MetadataRequest{ProviderTypeName: "k3s"}, resp)

	if got, want := resp.TypeName, "k3s_kubeconfig"; got != want {
		t.Errorf("TypeName = %q, want %q", got, want)
	}
}

func TestK3sKubeConfigResourceBinDirSchema(t *testing.T) {
	ctx := context.Background()
	r := &K3sKubeConfigResource{}
	resp := &frameworkresource.SchemaResponse{}
	r.Schema(ctx, frameworkresource.SchemaRequest{}, resp)

	attr, ok := resp.Schema.Attributes["bin_dir"].(resourceschema.StringAttribute)
	if !ok {
		t.Fatalf("bin_dir schema = %T, want resourceschema.StringAttribute", resp.Schema.Attributes["bin_dir"])
	}
	if !attr.IsOptional() {
		t.Error("bin_dir should be optional")
	}
	if !attr.IsComputed() {
		t.Error("bin_dir should be computed")
	}

	defResp := defaults.StringResponse{}
	attr.Default.DefaultString(ctx, defaults.StringRequest{}, &defResp)
	if got, want := defResp.PlanValue.ValueString(), k3s.BIN_DIR; got != want {
		t.Errorf("bin_dir default = %q, want %q", got, want)
	}
}

func TestK3sKubeConfigResourceModelRoundTripsBinDir(t *testing.T) {
	data := K3sKubeConfigResourceModel{
		BinDir: types.StringValue("/opt/bin"),
	}

	readData := data.toDataModel()
	if got, want := readData.BinDir.ValueString(), "/opt/bin"; got != want {
		t.Errorf("toDataModel().BinDir = %q, want %q", got, want)
	}

	var applied K3sKubeConfigResourceModel
	applied.applyDataModel(readData)
	if got, want := applied.BinDir.ValueString(), "/opt/bin"; got != want {
		t.Errorf("applyDataModel().BinDir = %q, want %q", got, want)
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
