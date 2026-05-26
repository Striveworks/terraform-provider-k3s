package provider

import (
	"context"
	"crypto/rand"
	"fmt"
	"math/big"
	"regexp"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

var (
	_ resource.Resource                = &K3sTokenResource{}
	_ resource.ResourceWithImportState = &K3sTokenResource{}
)

const (
	k3sTokenChars    = "0123456789abcdefghijklmnopqrstuvwxyz"
	k3sTokenIDLength = 6
	k3sTokenLength   = 32
)

var k3sTokenPattern = regexp.MustCompile(`^[a-z0-9]{32}$`)

type K3sTokenResource struct{}

type K3sTokenModel struct {
	Id    types.String `tfsdk:"id"`
	Token types.String `tfsdk:"token"`
}

func NewK3sTokenResource() resource.Resource {
	return &K3sTokenResource{}
}

func (r *K3sTokenResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_token"
}

func (r *K3sTokenResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Generates a K3s-compatible short server token suitable for bootstrapping the first server with `k3s_server.bootstrap_token`.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Public token ID, which is the first six characters of the generated token.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"token": schema.StringAttribute{
				Computed:            true,
				Sensitive:           true,
				MarkdownDescription: "Generated K3s-compatible short server token in `[a-z0-9]{32}` format.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}

func (r *K3sTokenResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	token, err := GenerateK3sToken()
	if err != nil {
		resp.Diagnostics.AddError("generating k3s token", err.Error())
		return
	}

	tflog.MaskMessageStrings(ctx, token)

	data := K3sTokenModel{
		Id:    types.StringValue(k3sTokenID(token)),
		Token: types.StringValue(token),
	}

	tflog.Info(ctx, "Created a k3s token resource")
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *K3sTokenResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data K3sTokenModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	token := data.Token.ValueString()
	if !IsValidK3sToken(token) {
		resp.Diagnostics.AddError("reading k3s token", fmt.Sprintf("state token %q does not match [a-z0-9]{32}", token))
		return
	}

	tflog.MaskMessageStrings(ctx, token)

	data.Id = types.StringValue(k3sTokenID(token))
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *K3sTokenResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data K3sTokenModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	token := data.Token.ValueString()
	if !IsValidK3sToken(token) {
		resp.Diagnostics.AddError("updating k3s token", fmt.Sprintf("state token %q does not match [a-z0-9]{32}", token))
		return
	}

	tflog.MaskMessageStrings(ctx, token)

	data.Id = types.StringValue(k3sTokenID(token))
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *K3sTokenResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	resp.State.RemoveResource(ctx)
}

func (r *K3sTokenResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	token := strings.TrimSpace(req.ID)
	if !IsValidK3sToken(token) {
		resp.Diagnostics.AddError("importing k3s token", fmt.Sprintf("import ID must match [a-z0-9]{32}, got %q", req.ID))
		return
	}

	tflog.MaskMessageStrings(ctx, token)

	data := K3sTokenModel{
		Id:    types.StringValue(k3sTokenID(token)),
		Token: types.StringValue(token),
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func GenerateK3sToken() (string, error) {
	return randomK3sTokenPart(k3sTokenLength)
}

func IsValidK3sToken(token string) bool {
	return k3sTokenPattern.MatchString(token)
}

func randomK3sTokenPart(length int) (string, error) {
	token := make([]byte, length)
	max_length := big.NewInt(int64(len(k3sTokenChars)))

	for i := range token {
		val, err := rand.Int(rand.Reader, max_length)
		if err != nil {
			return "", fmt.Errorf("could not generate random integer: %w", err)
		}
		token[i] = k3sTokenChars[val.Int64()]
	}

	return string(token), nil
}

func k3sTokenID(token string) string {
	if len(token) <= k3sTokenIDLength {
		return token
	}
	return token[:k3sTokenIDLength]
}
