package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"striveworks.us/terraform-provider-k3s/internal/k3s"
	"striveworks.us/terraform-provider-k3s/internal/schemas"
	"striveworks.us/terraform-provider-k3s/internal/ssh_client"
)

var _ datasource.DataSource = &K3sKubeConfigData{}

type K3sKubeConfigData struct{}

type K3sKubeConfigDataModel struct {
	Auth        types.Object `tfsdk:"auth"`
	ClusterAuth types.Object `tfsdk:"cluster_auth"`
	KubeConfig  types.String `tfsdk:"kubeconfig"`
	Hostname    types.String `tfsdk:"hostname"`
	K3sURL      types.String `tfsdk:"k3s_url"`
	AllowEmpty  types.Bool   `tfsdk:"allow_empty"`
}

func NewK3sKubeConfigData() datasource.DataSource {
	return &K3sKubeConfigData{}
}

// Metadata implements datasource.DataSource.
func (k *K3sKubeConfigData) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_kubeconfig"
}

// Read implements datasource.DataSource.
func (k *K3sKubeConfigData) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data K3sKubeConfigDataModel

	tflog.Trace(ctx, "Deserializing K3sKubeConfigDataModel")
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var sshConfig ssh_client.SSHConfig
	tflog.Trace(ctx, "Deserializing SSHConfig")
	resp.Diagnostics.Append(data.Auth.As(ctx, &sshConfig, basetypes.ObjectAsOptions{})...)
	if resp.Diagnostics.HasError() {
		return
	}

	normalizeKubeConfigDataSSHConfig(&sshConfig)

	tflog.Trace(ctx, "Validating SSHConfig")
	if err := sshConfig.Validate(); err != nil {
		resp.Diagnostics.AddError("validating auth", err.Error())
		return
	}

	sshClient, err := ssh_client.NewSSHClient(ctx, sshConfig)
	if err != nil {
		resp.Diagnostics.AddError("creating ssh client", err.Error())
		return
	}

	server := k3s.Server{}
	exists, _, err := server.Refresh(ctx, sshClient)
	if err != nil {
		if data.AllowEmpty.ValueBool() {
			tflog.Info(ctx, "allow_empty is true, returning null kubeconfig outputs")
			setEmptyKubeConfigData(ctx, &data, sshConfig)
			resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
			return
		}

		resp.Diagnostics.AddError("reading kubeconfig", err.Error())
		return
	}
	if !exists {
		if data.AllowEmpty.ValueBool() {
			tflog.Info(ctx, "k3s service is missing and allow_empty is true, returning null kubeconfig outputs")
			setEmptyKubeConfigData(ctx, &data, sshConfig)
			resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
			return
		}

		resp.Diagnostics.AddError("reading kubeconfig", fmt.Sprintf("no k3s service found on %s", sshClient.Host()))
		return
	}

	if err := populateKubeConfigData(ctx, &data, sshConfig, server.KubeConfig); err != nil {
		resp.Diagnostics.AddError("reading kubeconfig", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// Schema implements datasource.DataSource.
func (k *K3sKubeConfigData) Schema(_ context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: ("A utility for reading and manipulating kubeconfig. Common use case would be to extract " +
			"the auth credentials or override the server URL for a load balancer URL or DNS name."),
		Attributes: map[string]schema.Attribute{
			"auth":         ssh_client.SSHConfig{}.DataSourceSchema(),
			"cluster_auth": schemas.ClusterAuth{}.DataSourceSchema(),
			"allow_empty": schema.BoolAttribute{
				Optional:            true,
				MarkdownDescription: "If this is true, it will allow a missing kubeconfig and set null to all outputs",
			},
			"kubeconfig": schema.StringAttribute{
				Computed:            true,
				Sensitive:           true,
				MarkdownDescription: "Output of the kubeconfig from a k3s_server resource",
			},
			"hostname": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Override the api server's hostname",
			},
			"k3s_url": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "K3S_URL variable",
			},
		},
	}
}

func populateKubeConfigData(ctx context.Context, data *K3sKubeConfigDataModel, sshConfig ssh_client.SSHConfig, kubeconfig string) error {
	clusterAuth, err := schemas.BuildClusterAuth(kubeconfig)
	if err != nil {
		return fmt.Errorf("parsing kubeconfig: %w", err)
	}

	if !data.Hostname.IsNull() && !data.Hostname.IsUnknown() && data.Hostname.ValueString() != "" {
		clusterAuth.UpdateHost(data.Hostname.ValueString())
	}

	data.Auth = sshConfig.ToObject(ctx)
	data.ClusterAuth = clusterAuth.ToObject(ctx)
	data.KubeConfig = types.StringValue(clusterAuth.KubeConfig())
	data.K3sURL = clusterAuth.Server
	tflog.MaskMessageStrings(ctx, data.KubeConfig.ValueString())

	return nil
}

func setEmptyKubeConfigData(ctx context.Context, data *K3sKubeConfigDataModel, sshConfig ssh_client.SSHConfig) {
	data.Auth = sshConfig.ToObject(ctx)
	data.ClusterAuth = schemas.DefaultK3sClusterAuth()
	data.KubeConfig = types.StringNull()
	data.K3sURL = types.StringNull()
}

func normalizeKubeConfigDataSSHConfig(sshConfig *ssh_client.SSHConfig) {
	if sshConfig.Port.IsNull() || sshConfig.Port.IsUnknown() || sshConfig.Port.ValueInt32() == 0 {
		sshConfig.Port = types.Int32Value(22)
	}
	if sshConfig.IgnoreHostKeyVerification.IsNull() || sshConfig.IgnoreHostKeyVerification.IsUnknown() {
		sshConfig.IgnoreHostKeyVerification = types.BoolValue(false)
	}
}
