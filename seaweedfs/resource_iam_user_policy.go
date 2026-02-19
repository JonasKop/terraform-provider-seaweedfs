package seaweedfs

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource              = &iamUserPolicyResource{}
	_ resource.ResourceWithConfigure = &iamUserPolicyResource{}
)

func NewIAMUserPolicyResource() resource.Resource {
	return &iamUserPolicyResource{}
}

type iamUserPolicyResource struct {
	client *iamClient
	data   *providerData
}

type iamUserPolicyResourceModel struct {
	ID       types.String `tfsdk:"id"`
	UserName types.String `tfsdk:"user_name"`
	Name     types.String `tfsdk:"name"`
	Policy   types.String `tfsdk:"policy"`
}

func (r *iamUserPolicyResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_iam_user_policy"
}

func (r *iamUserPolicyResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages an inline IAM user policy in SeaweedFS.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed: true,
			},
			"user_name": schema.StringAttribute{
				Required: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"name": schema.StringAttribute{
				Required: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"policy": schema.StringAttribute{
				Required:    true,
				Description: "JSON policy document.",
			},
		},
	}
}

func (r *iamUserPolicyResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *iamUserPolicyResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan iamUserPolicyResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.data.withUserLock(plan.UserName.ValueString(), func() error {
		return r.client.PutUserPolicy(ctx, plan.UserName.ValueString(), plan.Name.ValueString(), plan.Policy.ValueString())
	}); err != nil {
		resp.Diagnostics.AddError("Failed to create IAM user policy", err.Error())
		return
	}

	state := iamUserPolicyResourceModel{
		ID:       types.StringValue(plan.UserName.ValueString() + ":" + plan.Name.ValueString()),
		UserName: types.StringValue(plan.UserName.ValueString()),
		Name:     types.StringValue(plan.Name.ValueString()),
		Policy:   types.StringValue(plan.Policy.ValueString()),
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *iamUserPolicyResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state iamUserPolicyResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	policy, err := r.client.GetUserPolicy(ctx, state.UserName.ValueString(), state.Name.ValueString())
	if err != nil {
		if isNoSuchEntityError(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Failed to read IAM user policy", err.Error())
		return
	}

	state.ID = types.StringValue(state.UserName.ValueString() + ":" + state.Name.ValueString())
	state.Policy = types.StringValue(policy)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *iamUserPolicyResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan iamUserPolicyResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.data.withUserLock(plan.UserName.ValueString(), func() error {
		return r.client.PutUserPolicy(ctx, plan.UserName.ValueString(), plan.Name.ValueString(), plan.Policy.ValueString())
	}); err != nil {
		resp.Diagnostics.AddError("Failed to update IAM user policy", err.Error())
		return
	}

	state := iamUserPolicyResourceModel{
		ID:       types.StringValue(plan.UserName.ValueString() + ":" + plan.Name.ValueString()),
		UserName: types.StringValue(plan.UserName.ValueString()),
		Name:     types.StringValue(plan.Name.ValueString()),
		Policy:   types.StringValue(plan.Policy.ValueString()),
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *iamUserPolicyResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state iamUserPolicyResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.data.withUserLock(state.UserName.ValueString(), func() error {
		return r.client.DeleteUserPolicy(ctx, state.UserName.ValueString(), state.Name.ValueString())
	}); err != nil && !isNoSuchEntityError(err) {
		resp.Diagnostics.AddError("Failed to delete IAM user policy", err.Error())
	}
}
