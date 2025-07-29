package handlers

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"striveworks.us/terraform-provider-k3s/internal/k3s"
)

type ServerClientModel struct {
	Auth types.Object `tfsdk:"auth"`
	// Configs
	BinDir      types.String `tfsdk:"bin_dir"`
	K3sConfig   types.String `tfsdk:"config"`
	K3sRegistry types.String `tfsdk:"registry"`
	// Highly Available config
	HaConfig types.Object `tfsdk:"highly_available"`
	// OIDC Support
	OidcConfig types.Object `tfsdk:"oidc"`
	// Outputs
	Id          types.String `tfsdk:"id"`
	Server      types.String `tfsdk:"server"`
	KubeConfig  types.String `tfsdk:"kubeconfig"`
	Token       types.String `tfsdk:"token"`
	Active      types.Bool   `tfsdk:"active"`
	ClusterAuth types.Object `tfsdk:"cluster_auth"`

	version    string
	haConfig   *HaConfig
	oidcConfig *OidcConfig
}

func (s *ServerClientModel) SetVersion(version *string) {
	if version != nil {
		s.version = *version
	}
}

func (s *ServerClientModel) ToServer(ctx context.Context) (k3s.Server, error) {
	server, err := k3s.NewK3sServerComponent(
		ctx,
		s.K3sConfig.ValueString(),
		s.K3sRegistry.ValueString(),
		s.version,
		s.BinDir.ValueString(),
	)

	if err != nil {
		return nil, err
	}

	if !s.HaConfig.IsNull() {
		cfg := NewHaConfig(ctx, s.HaConfig)
		s.haConfig = &cfg
		s.haConfig.configureServer(server)
	}

	if !s.OidcConfig.IsNull() {
		cfg := NewOidcConfig(ctx, s.OidcConfig)
		s.oidcConfig = &cfg
		s.oidcConfig.configureServer(server)
	}

	return server, nil
}

type TServerSSH interface {
	K3sTypeSSH
	K3sTypeToObject
}

type TK3sServerRead interface {
	k3s.ComponentResync
	k3s.ComponentStatus
	k3s.ComponentToken
	k3s.ServerKubeconfig
	k3s.ServerOidc
}

func (s *ServerClientModel) Read(
	ctx context.Context,
	auth TServerSSH,
	server TK3sServerRead,
) error {
	sshClient, err := auth.SshClient(ctx)
	if err != nil {
		return fmt.Errorf("creating ssh config: %s", err.Error())
	}
	tflog.Debug(ctx, "k3s server ssh client created")

	if err := server.Resync(sshClient); err != nil {
		return fmt.Errorf("error resyncing server: %s", err.Error())
	}
	tflog.Debug(ctx, "k3s server resynced")

	status, err := server.Status(sshClient)
	if err != nil {
		return fmt.Errorf("fetching status or status logs: %s", err.Error())
	}

	clusterAuth, err := BuildClusterAuth(server.KubeConfig())
	if err != nil {
		return fmt.Errorf("fetching cluster auth: %s", err.Error())
	}

	if s.oidcConfig != nil {
		if err := s.oidcConfig.setJwks(sshClient); err != nil {
			return err
		}
		tflog.Debug(ctx, "k3s server fetched jwks")
		s.OidcConfig = s.oidcConfig.ToObject(ctx)
	}
	if s.haConfig != nil {
		s.HaConfig = s.haConfig.ToObject(ctx)
	}

	s.ClusterAuth = clusterAuth.ToObject(ctx)
	s.Active = types.BoolValue(status)
	s.KubeConfig = types.StringValue(server.KubeConfig())
	s.Token = types.StringValue(server.Token())

	return nil
}

type TK3sServerDelete interface {
	k3s.ComponentUninstall
}

func (s *ServerClientModel) Delete(
	ctx context.Context,
	auth TServerSSH,
	server TK3sServerDelete,
) error {
	sshClient, err := auth.SshClient(ctx)
	if err != nil {
		return fmt.Errorf("creating ssh config: %s", err.Error())
	}
	tflog.Debug(ctx, "k3s server ssh client created")

	// If single node skip uninstalling node

	if err := server.Uninstall(sshClient, s.K3sConfig.ValueString()); err != nil {
		return fmt.Errorf("error resyncing server: %s", err.Error())
	}
	tflog.Debug(ctx, "k3s server uninstalled")

	return nil
}

type TServerCreate interface {
	k3s.ComponentPreInstall
	k3s.ComponentInstall
	k3s.ComponentStatus
	k3s.ServerKubeconfig
	k3s.ComponentToken
	k3s.ServerOidc
}

func (s *ServerClientModel) Create(
	ctx context.Context,
	auth TServerSSH,
	server TServerCreate,
) error {
	sshClient, err := auth.SshClient(ctx)
	if err != nil {
		return fmt.Errorf("creating ssh config: %s", err.Error())
	}
	tflog.Debug(ctx, "k3s server ssh client created")

	if err := server.Preinstall(sshClient); err != nil {
		return fmt.Errorf("running k3s server prereqs: %s", err.Error())
	}
	tflog.Debug(ctx, "k3s server pre install ran")

	if err := server.Install(sshClient); err != nil {
		return fmt.Errorf("running k3s server prereqs: %s", err.Error())
	}
	tflog.Debug(ctx, "k3s server install ran")

	status, err := server.Status(sshClient)
	if err != nil {
		return fmt.Errorf("fetching status or status logs: %s", err.Error())
	}
	tflog.Debug(ctx, "k3s server status ran")

	clusterAuth, err := BuildClusterAuth(server.KubeConfig())
	if err != nil {
		return fmt.Errorf("fetching status kubeconfig: %s", err.Error())
	}
	clusterAuth.UpdateHost(sshClient.HostnameOrIpAddress())

	if s.oidcConfig != nil {
		if err := s.oidcConfig.setJwks(sshClient); err != nil {
			return err
		}
		tflog.Debug(ctx, "k3s server fetched jwks")
		s.OidcConfig = s.oidcConfig.ToObject(ctx)
	}
	if s.haConfig != nil {
		s.HaConfig = s.haConfig.ToObject(ctx)
	}

	s.ClusterAuth = clusterAuth.ToObject(ctx)
	s.KubeConfig = types.StringValue(clusterAuth.KubeConfig())
	s.Token = types.StringValue(server.Token())
	s.Id = types.StringValue(fmt.Sprintf("server,%s", sshClient.HostnameOrIpAddress()))
	s.Server = clusterAuth.Server
	s.Active = types.BoolValue(status)

	return nil
}

type TServerUpdate interface {
	k3s.ComponentPreInstall
	k3s.ComponentUpdate
	k3s.ComponentStatus
	k3s.ServerKubeconfig
	k3s.ComponentToken
	k3s.ServerOidc
}

func (s *ServerClientModel) Update(
	ctx context.Context,
	inc ServerClientModel,
	auth TServerSSH,
	server TServerUpdate,
) error {
	sshClient, err := auth.SshClient(ctx)
	if err != nil {
		return fmt.Errorf("creating ssh config: %s", err.Error())
	}
	tflog.Debug(ctx, "k3s server ssh client created")

	if s.K3sConfig.Equal(inc.K3sConfig) && s.K3sRegistry.Equal(inc.K3sRegistry) && s.OidcConfig.Equal(inc.OidcConfig) {
		tflog.Debug(ctx, "No change is needed, only supporting config, registry and oidc updates")
		return nil
	}

	if err := server.Preinstall(sshClient); err != nil {
		return fmt.Errorf("running k3s server prereqs: %s", err.Error())
	}
	tflog.Debug(ctx, "k3s server pre install ran")

	if err := server.Update(sshClient); err != nil {
		return fmt.Errorf("k3s agent updating: %s", err.Error())
	}
	tflog.Debug(ctx, "k3s server update ran")

	status, err := server.Status(sshClient)
	if err != nil {
		return fmt.Errorf("fetching status or status logs: %s", err.Error())
	}
	tflog.Debug(ctx, "k3s server status ran")

	if s.oidcConfig != nil {
		if err := s.oidcConfig.setJwks(sshClient); err != nil {
			return err
		}
		tflog.Debug(ctx, "k3s server fetched jwks")
		s.OidcConfig = s.oidcConfig.ToObject(ctx)
	}

	if s.haConfig != nil {
		s.HaConfig = s.haConfig.ToObject(ctx)
	}

	s.Active = types.BoolValue(status)
	s.K3sRegistry = inc.K3sRegistry
	s.K3sConfig = inc.K3sConfig

	return nil
}
