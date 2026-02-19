package seaweedfs

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource                = &iamAccessKeyResource{}
	_ resource.ResourceWithConfigure   = &iamAccessKeyResource{}
	_ resource.ResourceWithImportState = &iamAccessKeyResource{}
)

func NewIAMAccessKeyResource() resource.Resource {
	return &iamAccessKeyResource{}
}

type iamAccessKeyResource struct {
	client *iamClient
	data   *providerData
}

type iamAccessKeyResourceModel struct {
	ID              types.String `tfsdk:"id"`
	UserName        types.String `tfsdk:"user_name"`
	AccessKeyID     types.String `tfsdk:"access_key_id"`
	SecretAccessKey types.String `tfsdk:"secret_access_key"`
	Status          types.String `tfsdk:"status"`
}

func (r *iamAccessKeyResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_iam_access_key"
}

func (r *iamAccessKeyResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a SeaweedFS IAM access key for a user.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{Computed: true},
			"user_name": schema.StringAttribute{
				Required: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"access_key_id": schema.StringAttribute{
				Computed: true,
			},
			"secret_access_key": schema.StringAttribute{
				Computed:  true,
				Sensitive: true,
			},
			"status": schema.StringAttribute{
				Computed: true,
			},
		},
	}
}

func (r *iamAccessKeyResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	data, ok := req.ProviderData.(*providerData)
	if !ok {
		resp.Diagnostics.AddError("Unexpected provider data type", fmt.Sprintf("Expected *providerData, got %T", req.ProviderData))
		return
	}
	r.client = data.client
	r.data = data
}

func (r *iamAccessKeyResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan iamAccessKeyResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var key iamAccessKey
	err := r.data.withUserLock(plan.UserName.ValueString(), func() error {
		var innerErr error
		key, innerErr = r.client.CreateAccessKey(ctx, plan.UserName.ValueString())
		return innerErr
	})
	if err != nil {
		resp.Diagnostics.AddError("Failed to create IAM access key", err.Error())
		return
	}

	state := iamAccessKeyResourceModel{
		ID:              types.StringValue(key.AccessKeyID),
		UserName:        types.StringValue(plan.UserName.ValueString()),
		AccessKeyID:     types.StringValue(key.AccessKeyID),
		SecretAccessKey: types.StringValue(key.SecretAccessKey),
		Status:          types.StringValue(key.Status),
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *iamAccessKeyResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state iamAccessKeyResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	keys, err := r.client.ListAccessKeys(ctx, state.UserName.ValueString())
	if err != nil {
		if isNoSuchEntityError(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Failed to read IAM access key", err.Error())
		return
	}

	found := false
	status := ""
	for _, key := range keys {
		if key.AccessKeyID == state.AccessKeyID.ValueString() {
			found = true
			status = key.Status
			break
		}
	}
	if !found {
		resp.State.RemoveResource(ctx)
		return
	}

	state.ID = types.StringValue(state.AccessKeyID.ValueString())
	state.Status = types.StringValue(status)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *iamAccessKeyResource) Update(_ context.Context, _ resource.UpdateRequest, resp *resource.UpdateResponse) {
	resp.Diagnostics.AddError("Update not supported", "seaweedfs_iam_access_key supports replacement only.")
}

func (r *iamAccessKeyResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state iamAccessKeyResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.data.withUserLock(state.UserName.ValueString(), func() error {
		return r.client.DeleteAccessKey(ctx, state.UserName.ValueString(), state.AccessKeyID.ValueString())
	}); err != nil && !isNoSuchEntityError(err) {
		resp.Diagnostics.AddError("Failed to delete IAM access key", err.Error())
	}
}

func (r *iamAccessKeyResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.Split(req.ID, ",")
	if len(parts) != 2 {
		resp.Diagnostics.AddError("Invalid import ID", "Expected import id in format `user_name,access_key_id`.")
		return
	}

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), parts[1])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("user_name"), parts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("access_key_id"), parts[1])...)
}
