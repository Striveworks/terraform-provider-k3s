package handlers

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"striveworks.us/terraform-provider-k3s/internal/k3s"
)

type K3sKubeConfig struct {
	Auth        types.Object `tfsdk:"auth"`
	ClusterAuth types.Object `tfsdk:"cluster_auth"`
	KubeConfig  types.String `tfsdk:"kubeconfig"`
	Hostname    types.String `tfsdk:"hostname"`
	AllowEmpty  types.Bool   `tfsdk:"allow_empty"`
}

type KubeConfig struct {
	Auth        types.Object `tfsdk:"auth"`
	ClusterAuth types.Object `tfsdk:"cluster_auth"`
	KubeConfig  types.String `tfsdk:"kubeconfig"`
	Hostname    types.String `tfsdk:"hostname"`
	AllowEmpty  types.Bool   `tfsdk:"allow_empty"`
}

type TKubeConfigRead interface {
	K3sTypeSSH
	K3sTypeToObject
}

type TK3SServerRead interface {
	k3s.ServerKubeconfig
	k3s.ComponentResync
}

// Performs the read operation on the k3s_kubeconfig data source. This method allows testable decoupling from
// terraform operations.
func (s *K3sKubeConfig) Read(ctx context.Context, auth TKubeConfigRead, server TK3SServerRead) error {
	sshClient, err := auth.SshClient(ctx)
	if err != nil {
		return fmt.Errorf("creating ssh config: %s", err.Error())
	}

	if err := server.Resync(sshClient); err != nil {
		if s.AllowEmpty.ValueBool() {
			tflog.Info(ctx, "Allowing empty kubeconfig, returning nulls")
			s.setDefaults()
			return nil
		}

		return fmt.Errorf("error resyncing server: %s", err.Error())
	}

	clusterAuth, err := BuildClusterAuth(server.KubeConfig())
	if err != nil {
		return fmt.Errorf("parsing kubeconfig: %s", err.Error())
	}

	// Set hostname
	if !s.Hostname.IsNull() {
		clusterAuth.UpdateHost(s.Hostname.ValueString())
	}

	s.Auth = auth.ToObject(ctx)
	s.ClusterAuth = clusterAuth.ToObject(ctx)
	s.KubeConfig = types.StringValue(clusterAuth.KubeConfig())

	return nil
}

func (s *K3sKubeConfig) setDefaults() {
	s.Auth = DefaultNodeAuth()
	s.ClusterAuth = DefaultK3sClusterAuth()
	s.KubeConfig = types.StringNull()
	s.AllowEmpty = types.BoolValue(false)
}
