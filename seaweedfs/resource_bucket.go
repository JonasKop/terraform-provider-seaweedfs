package seaweedfs

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource                = &bucketResource{}
	_ resource.ResourceWithConfigure   = &bucketResource{}
	_ resource.ResourceWithImportState = &bucketResource{}
)

func NewBucketResource() resource.Resource {
	return &bucketResource{}
}

type bucketResource struct {
	client *iamClient
}

type bucketResourceModel struct {
	ID     types.String `tfsdk:"id"`
	Bucket types.String `tfsdk:"bucket"`
	ARN    types.String `tfsdk:"arn"`
	Tags   types.Map    `tfsdk:"tags"`
}

func (r *bucketResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_bucket"
}

func (r *bucketResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a SeaweedFS S3 bucket.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed: true,
			},
			"bucket": schema.StringAttribute{
				Required:    true,
				Description: "Bucket name.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"arn": schema.StringAttribute{
				Computed:    true,
				Description: "Bucket ARN.",
			},
			"tags": schema.MapAttribute{
				Optional:    true,
				Computed:    true,
				ElementType: types.StringType,
				Description: "Bucket tags.",
			},
		},
	}
}

func (r *bucketResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	data, ok := req.ProviderData.(*providerData)
	if !ok {
		resp.Diagnostics.AddError("Unexpected provider data type", fmt.Sprintf("Expected *providerData, got %T", req.ProviderData))
		return
	}
	r.client = data.client
}

func (r *bucketResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan bucketResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.client.CreateBucket(ctx, plan.Bucket.ValueString()); err != nil {
		if !isBucketAlreadyExistsError(err) {
			resp.Diagnostics.AddError("Failed to create bucket", err.Error())
			return
		}

		if headErr := r.client.HeadBucket(ctx, plan.Bucket.ValueString()); headErr != nil {
			resp.Diagnostics.AddError("Failed to verify existing bucket", headErr.Error())
			return
		}
	}

	planTags, diags := stringMapFromTerraformMap(ctx, plan.Tags)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	if len(planTags) > 0 {
		if err := r.client.PutBucketTags(ctx, plan.Bucket.ValueString(), planTags); err != nil {
			resp.Diagnostics.AddError("Failed to set bucket tags", err.Error())
			return
		}
	}

	remoteTags, err := r.client.GetBucketTags(ctx, plan.Bucket.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Failed to read bucket tags", err.Error())
		return
	}

	tagsValue, diags := terraformMapFromStringMap(ctx, remoteTags)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	state := bucketResourceModel{
		ID:     types.StringValue(plan.Bucket.ValueString()),
		Bucket: types.StringValue(plan.Bucket.ValueString()),
		ARN:    types.StringValue("arn:aws:s3:::" + plan.Bucket.ValueString()),
		Tags:   tagsValue,
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *bucketResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state bucketResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.client.HeadBucket(ctx, state.Bucket.ValueString()); err != nil {
		if isNoSuchBucketError(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Failed to read bucket", err.Error())
		return
	}

	tags, err := r.client.GetBucketTags(ctx, state.Bucket.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Failed to read bucket tags", err.Error())
		return
	}
	tagsValue, diags := terraformMapFromStringMap(ctx, tags)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	state.ID = types.StringValue(state.Bucket.ValueString())
	state.ARN = types.StringValue("arn:aws:s3:::" + state.Bucket.ValueString())
	state.Tags = tagsValue
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *bucketResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan bucketResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	tags, diags := stringMapFromTerraformMap(ctx, plan.Tags)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	if len(tags) == 0 {
		if err := r.client.DeleteBucketTags(ctx, plan.Bucket.ValueString()); err != nil && !isNoSuchBucketError(err) {
			resp.Diagnostics.AddError("Failed to delete bucket tags", err.Error())
			return
		}
	} else {
		if err := r.client.PutBucketTags(ctx, plan.Bucket.ValueString(), tags); err != nil {
			resp.Diagnostics.AddError("Failed to update bucket tags", err.Error())
			return
		}
	}

	remoteTags, err := r.client.GetBucketTags(ctx, plan.Bucket.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Failed to read bucket tags", err.Error())
		return
	}
	tagsValue, diags := terraformMapFromStringMap(ctx, remoteTags)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	state := bucketResourceModel{
		ID:     types.StringValue(plan.Bucket.ValueString()),
		Bucket: types.StringValue(plan.Bucket.ValueString()),
		ARN:    types.StringValue("arn:aws:s3:::" + plan.Bucket.ValueString()),
		Tags:   tagsValue,
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *bucketResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state bucketResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.client.DeleteBucket(ctx, state.Bucket.ValueString()); err != nil && !isNoSuchBucketError(err) {
		resp.Diagnostics.AddError("Failed to delete bucket", err.Error())
	}
}

func (r *bucketResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("bucket"), req.ID)...)
}

func stringMapFromTerraformMap(ctx context.Context, value types.Map) (map[string]string, diag.Diagnostics) {
	if value.IsNull() || value.IsUnknown() {
		return map[string]string{}, nil
	}

	out := map[string]string{}
	diags := value.ElementsAs(ctx, &out, false)
	return out, diags
}

func terraformMapFromStringMap(ctx context.Context, values map[string]string) (types.Map, diag.Diagnostics) {
	if values == nil {
		values = map[string]string{}
	}
	return types.MapValueFrom(ctx, types.StringType, values)
}
