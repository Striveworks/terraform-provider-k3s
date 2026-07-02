package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int32default"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"striveworks.us/terraform-provider-k3s/internal/schemas"
)

var _ resource.Resource = &K3sKubeConfigResource{}

type K3sKubeConfigResource struct{}

type K3sKubeConfigResourceModel struct {
	Id          types.String `tfsdk:"id"`
	Auth        types.Object `tfsdk:"auth"`
	ClusterAuth types.Object `tfsdk:"cluster_auth"`
	KubeConfig  types.String `tfsdk:"kubeconfig"`
	Hostname    types.String `tfsdk:"hostname"`
	K3sURL      types.String `tfsdk:"k3s_url"`
	AllowEmpty  types.Bool   `tfsdk:"allow_empty"`
}

func NewK3sKubeConfigResource() resource.Resource {
	return &K3sKubeConfigResource{}
}

func (k *K3sKubeConfigResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_kubeconfig"
}

func (k *K3sKubeConfigResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: ("Reads and normalizes kubeconfig as a managed resource. Use this instead of the " +
			"`k3s_kubeconfig` data source when downstream resources should keep the current kubeconfig-derived values " +
			"until refresh observes that the remote kubeconfig changed."),
		Attributes: map[string]schema.Attribute{
			"auth":         kubeConfigResourceSSHSchema(),
			"cluster_auth": schemas.ClusterAuth{}.Schema(),
			"allow_empty": schema.BoolAttribute{
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(false),
				MarkdownDescription: "If this is true, it will allow a missing kubeconfig and set null to all outputs",
			},
			"hostname": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Override the api server's hostname",
			},
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "SSH target used to read the kubeconfig.",
			},
			"kubeconfig": schema.StringAttribute{
				Computed:            true,
				Sensitive:           true,
				MarkdownDescription: "Output of the kubeconfig from a k3s_server resource",
			},
			"k3s_url": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "K3S_URL variable",
			},
		},
	}
}

func (k *K3sKubeConfigResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data K3sKubeConfigResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	k.read(ctx, &data, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Info(ctx, "Created a k3s kubeconfig resource")
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (k *K3sKubeConfigResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data K3sKubeConfigResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	k.read(ctx, &data, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (k *K3sKubeConfigResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data K3sKubeConfigResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	k.read(ctx, &data, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (k *K3sKubeConfigResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	resp.State.RemoveResource(ctx)
}

func (k *K3sKubeConfigResource) read(ctx context.Context, data *K3sKubeConfigResourceModel, diags *diag.Diagnostics) {
	readData := data.toDataModel()

	id, readDiags := readKubeConfig(ctx, &readData)
	diags.Append(readDiags...)
	if diags.HasError() {
		return
	}

	data.applyDataModel(readData)
	data.Id = id
}

func (data K3sKubeConfigResourceModel) toDataModel() K3sKubeConfigDataModel {
	return K3sKubeConfigDataModel{
		Auth:        data.Auth,
		ClusterAuth: data.ClusterAuth,
		KubeConfig:  data.KubeConfig,
		Hostname:    data.Hostname,
		K3sURL:      data.K3sURL,
		AllowEmpty:  data.AllowEmpty,
	}
}

func (data *K3sKubeConfigResourceModel) applyDataModel(readData K3sKubeConfigDataModel) {
	data.Auth = readData.Auth
	data.ClusterAuth = readData.ClusterAuth
	data.KubeConfig = readData.KubeConfig
	data.Hostname = readData.Hostname
	data.K3sURL = readData.K3sURL
	data.AllowEmpty = readData.AllowEmpty
}

func kubeConfigResourceSSHSchema() schema.Attribute {
	return schema.SingleNestedAttribute{
		Required: true,
		Description: `SSH authentication config. At least one of password, private_key, or private_key_file must be provided.
		If multiple credential types are provided, each is added to the SSH auth methods.
		For host key verification, host_key or host_key_file can be passed in, otherwise host key verification is ignored.
		`,
		Attributes: map[string]schema.Attribute{
			"user": schema.StringAttribute{
				Required:            true,
				Sensitive:           false,
				MarkdownDescription: "SSH User",
			},
			"host": schema.StringAttribute{
				Required:            true,
				Sensitive:           false,
				MarkdownDescription: "Hostname or IP Address",
			},
			"port": schema.Int32Attribute{
				Optional:            true,
				Computed:            true,
				Sensitive:           false,
				MarkdownDescription: "SSH Port. Defaults to 22 when omitted.",
				Default:             int32default.StaticInt32(22),
			},
			"private_key": schema.StringAttribute{
				Optional:            true,
				Sensitive:           true,
				MarkdownDescription: "Inline private key in PEM format",
			},
			"private_key_file": schema.StringAttribute{
				Optional:            true,
				Sensitive:           true,
				MarkdownDescription: "Path to pem file",
			},
			"password": schema.StringAttribute{
				Optional:            true,
				Sensitive:           true,
				MarkdownDescription: "SSH Password",
			},
			"host_key": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Inline SSH host public key",
			},
			"host_key_file": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Path to SSH host public key",
			},
		},
	}
}
