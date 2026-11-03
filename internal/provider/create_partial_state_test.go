package provider

import (
	"context"
	"testing"

	fwresource "github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"striveworks.us/terraform-provider-k3s/internal/k3s"
	"striveworks.us/terraform-provider-k3s/internal/schemas"
	"striveworks.us/terraform-provider-k3s/internal/ssh_client"
)

func testSSHAuthObject(ctx context.Context) types.Object {
	sshConfig := ssh_client.SSHConfig{
		User:     types.StringValue("root"),
		Host:     types.StringValue("192.0.2.10"),
		Port:     types.Int32Value(22),
		Password: types.StringValue("s3cr3t"),
	}
	return sshConfig.ToObject(ctx)
}

func TestSaveServerPartialState(t *testing.T) {
	ctx := context.Background()

	schemaResp := fwresource.SchemaResponse{}
	(&K3sServerResource{}).Schema(ctx, fwresource.SchemaRequest{}, &schemaResp)

	oidcConfig := &schemas.OidcConfig{
		Audience:     types.StringValue("aud"),
		SigningPKCS8: types.StringValue("pkcs8"),
		SigningKey:   types.StringValue("key"),
		Issuer:       types.StringValue("https://issuer.example.com"),
		JWKSKeys:     types.StringUnknown(),
	}

	// Model as it looks when Create fails right after Install: every computed
	// attribute still carries its unknown planned value.
	data := ServerClientModel{
		Version:        types.StringUnknown(),
		Auth:           testSSHAuthObject(ctx),
		BinDir:         types.StringValue(k3s.BIN_DIR),
		K3sConfig:      types.StringNull(),
		K3sRegistry:    types.StringNull(),
		Env:            types.MapNull(types.StringType),
		HaConfig:       types.ObjectNull(schemas.HaConfig{}.AttributeTypes()),
		OidcConfig:     oidcConfig.ToObject(ctx),
		BootstrapToken: types.StringNull(),
		Orphan:         types.BoolValue(false),
		Id:             types.StringUnknown(),
		Server:         types.StringUnknown(),
		KubeConfig:     types.StringUnknown(),
		Token:          types.StringValue("observed-token"),
		Active:         types.BoolUnknown(),
		ClusterAuth:    types.ObjectUnknown(schemas.ClusterAuth{}.AttributeTypes()),
	}

	resp := &fwresource.CreateResponse{State: tfsdk.State{Schema: schemaResp.Schema}}
	server := k3s.Server{Version: "v1.33.1+k3s1"}
	sshClient := ssh_client.SSHClient{HostnameOrIPAddress: "192.0.2.10", Port: 22}

	saveServerPartialState(ctx, resp, &data, server, oidcConfig, sshClient)

	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected diagnostics: %v", resp.Diagnostics)
	}
	if resp.State.Raw.IsNull() {
		t.Fatal("expected partial state to be set, got null state")
	}
	if !resp.State.Raw.IsFullyKnown() {
		t.Fatalf("partial state contains unknown values: %v", resp.State.Raw)
	}

	if got := data.Id.ValueString(); got != "192.0.2.10:22" {
		t.Errorf("Id = %q, want %q", got, "192.0.2.10:22")
	}
	if got := data.Version.ValueString(); got != "v1.33.1+k3s1" {
		t.Errorf("Version = %q, want %q", got, "v1.33.1+k3s1")
	}
	if !data.KubeConfig.IsNull() {
		t.Errorf("KubeConfig = %v, want null", data.KubeConfig)
	}
	// Already-known values must be preserved, not overwritten.
	if got := data.Token.ValueString(); got != "observed-token" {
		t.Errorf("Token = %q, want %q", got, "observed-token")
	}
	if !data.Server.IsNull() {
		t.Errorf("Server = %v, want null", data.Server)
	}
	if data.Active.ValueBool() {
		t.Errorf("Active = %v, want false", data.Active)
	}
	if !data.ClusterAuth.IsNull() {
		t.Errorf("ClusterAuth = %v, want null", data.ClusterAuth)
	}
	if data.OidcConfig.IsNull() || data.OidcConfig.IsUnknown() {
		t.Fatalf("OidcConfig = %v, want known object", data.OidcConfig)
	}
	if jwks := data.OidcConfig.Attributes()["jwks_keys"]; !jwks.IsNull() {
		t.Errorf("OidcConfig jwks_keys = %v, want null", jwks)
	}
}

func TestSaveAgentPartialState(t *testing.T) {
	ctx := context.Background()

	schemaResp := fwresource.SchemaResponse{}
	(&K3sAgentResource{}).Schema(ctx, fwresource.SchemaRequest{}, &schemaResp)

	// Model as it looks when Create fails right after Install: every computed
	// attribute still carries its unknown planned value.
	data := AgentClientModel{
		Version:     types.StringUnknown(),
		Auth:        testSSHAuthObject(ctx),
		BinDir:      types.StringValue(k3s.BIN_DIR),
		K3sConfig:   types.StringNull(),
		K3sRegistry: types.StringNull(),
		Env:         types.MapNull(types.StringType),
		Server:      types.StringValue("https://192.0.2.1:6443"),
		Token:       types.StringValue("join-token"),
		Orphan:      types.BoolValue(false),
		Id:          types.StringUnknown(),
		Active:      types.BoolUnknown(),
	}

	resp := &fwresource.CreateResponse{State: tfsdk.State{Schema: schemaResp.Schema}}
	agent := k3s.Agent{}
	sshClient := ssh_client.SSHClient{HostnameOrIPAddress: "192.0.2.20", Port: 22}

	saveAgentPartialState(ctx, resp, &data, agent, sshClient)

	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected diagnostics: %v", resp.Diagnostics)
	}
	if resp.State.Raw.IsNull() {
		t.Fatal("expected partial state to be set, got null state")
	}
	if !resp.State.Raw.IsFullyKnown() {
		t.Fatalf("partial state contains unknown values: %v", resp.State.Raw)
	}

	if got := data.Id.ValueString(); got != "192.0.2.20:22" {
		t.Errorf("Id = %q, want %q", got, "192.0.2.20:22")
	}
	if !data.Version.IsNull() {
		t.Errorf("Version = %v, want null", data.Version)
	}
	if data.Active.ValueBool() {
		t.Errorf("Active = %v, want false", data.Active)
	}
}
