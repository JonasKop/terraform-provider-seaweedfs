package seaweedfs

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource                = &iamUserResource{}
	_ resource.ResourceWithConfigure   = &iamUserResource{}
	_ resource.ResourceWithImportState = &iamUserResource{}
)

func NewIAMUserResource() resource.Resource {
	return &iamUserResource{}
}

type iamUserResource struct {
	client *iamClient
	data   *providerData
}

type iamUserResourceModel struct {
	ID     types.String `tfsdk:"id"`
	Name   types.String `tfsdk:"name"`
	Path   types.String `tfsdk:"path"`
	ARN    types.String `tfsdk:"arn"`
	UserID types.String `tfsdk:"user_id"`
}

func (r *iamUserResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_iam_user"
}

func (r *iamUserResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a SeaweedFS IAM user using IAM query API calls.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "Terraform identifier for this resource. Equals user name.",
			},
			"name": schema.StringAttribute{
				Required:    true,
				Description: "IAM user name.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"path": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Default:     stringdefault.StaticString("/"),
				Description: "IAM path for the user.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"arn": schema.StringAttribute{
				Computed:    true,
				Description: "ARN returned by SeaweedFS.",
			},
			"user_id": schema.StringAttribute{
				Computed:    true,
				Description: "Unique user identifier returned by SeaweedFS.",
			},
		},
	}
}

func (r *iamUserResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	data, ok := req.ProviderData.(*providerData)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected provider data type",
			fmt.Sprintf("Expected *providerData, got %T", req.ProviderData),
		)
		return
	}
	r.client = data.client
	r.data = data
}

func (r *iamUserResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan iamUserResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var user getUserResponse
	err := r.data.withUserLock(plan.Name.ValueString(), func() error {
		var innerErr error
		user, innerErr = r.client.CreateUser(ctx, plan.Name.ValueString(), plan.Path.ValueString())
		return innerErr
	})
	if err != nil {
		resp.Diagnostics.AddError(
			"Failed to create IAM user",
			err.Error(),
		)
		return
	}

	userPath := user.User.Path
	if userPath == "" {
		userPath = plan.Path.ValueString()
		if userPath == "" {
			userPath = "/"
		}
	}

	state := iamUserResourceModel{
		ID:     types.StringValue(user.User.UserName),
		Name:   types.StringValue(user.User.UserName),
		Path:   types.StringValue(userPath),
		ARN:    types.StringValue(user.User.Arn),
		UserID: types.StringValue(user.User.UserID),
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *iamUserResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state iamUserResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	user, err := r.client.GetUser(ctx, state.Name.ValueString())
	if err != nil {
		if isNoSuchEntityError(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError(
			"Failed to read IAM user",
			err.Error(),
		)
		return
	}

	userPath := user.User.Path
	if userPath == "" {
		userPath = state.Path.ValueString()
		if userPath == "" {
			userPath = "/"
		}
	}

	state.ID = types.StringValue(user.User.UserName)
	state.Name = types.StringValue(user.User.UserName)
	state.Path = types.StringValue(userPath)
	state.ARN = types.StringValue(user.User.Arn)
	state.UserID = types.StringValue(user.User.UserID)

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *iamUserResource) Update(_ context.Context, _ resource.UpdateRequest, resp *resource.UpdateResponse) {
	resp.Diagnostics.AddError(
		"Update not supported",
		"seaweedfs_iam_user currently supports replacement on changes to name/path only.",
	)
}

func (r *iamUserResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state iamUserResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.data.withUserLock(state.Name.ValueString(), func() error {
		return r.client.DeleteUser(ctx, state.Name.ValueString())
	}); err != nil && !isNoSuchEntityError(err) {
		resp.Diagnostics.AddError(
			"Failed to delete IAM user",
			err.Error(),
		)
	}
}

func (r *iamUserResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("name"), req.ID)...)
}
