package seaweedfs

import (
	"context"
	"fmt"
	"sync"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ provider.Provider = &seaweedfsProvider{}

func NewProvider() provider.Provider {
	return &seaweedfsProvider{}
}

type seaweedfsProvider struct{}

type seaweedfsProviderModel struct {
	Endpoint  types.String `tfsdk:"endpoint"`
	Region    types.String `tfsdk:"region"`
	AccessKey types.String `tfsdk:"access_key"`
	SecretKey types.String `tfsdk:"secret_key"`
	Insecure  types.Bool   `tfsdk:"insecure"`
}

type providerData struct {
	client    *iamClient
	iamWrite  sync.Mutex
	lockMu    sync.Mutex
	userLocks map[string]*sync.Mutex
}

func (d *providerData) withUserLock(userName string, fn func() error) error {
	d.iamWrite.Lock()
	defer d.iamWrite.Unlock()

	lock := d.getUserLock(userName)
	lock.Lock()
	defer lock.Unlock()
	return fn()
}

func (d *providerData) getUserLock(userName string) *sync.Mutex {
	d.lockMu.Lock()
	defer d.lockMu.Unlock()

	if d.userLocks == nil {
		d.userLocks = map[string]*sync.Mutex{}
	}

	if lock, ok := d.userLocks[userName]; ok {
		return lock
	}

	lock := &sync.Mutex{}
	d.userLocks[userName] = lock
	return lock
}

func (p *seaweedfsProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "seaweedfs"
}

func (p *seaweedfsProvider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"endpoint": schema.StringAttribute{
				Required:    true,
				Description: "SeaweedFS S3/IAM endpoint, for example https://s3.example.com",
			},
			"region": schema.StringAttribute{
				Optional:    true,
				Description: "Signing region for AWS SigV4. Default: us-east-1.",
			},
			"access_key": schema.StringAttribute{
				Required:    true,
				Sensitive:   true,
				Description: "Admin access key used to manage SeaweedFS IAM users.",
			},
			"secret_key": schema.StringAttribute{
				Required:    true,
				Sensitive:   true,
				Description: "Admin secret key used to manage SeaweedFS IAM users.",
			},
			"insecure": schema.BoolAttribute{
				Optional:    true,
				Description: "If true, skip TLS certificate verification.",
			},
		},
	}
}

func (p *seaweedfsProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var config seaweedfsProviderModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	region := "us-east-1"
	if !config.Region.IsNull() && !config.Region.IsUnknown() && config.Region.ValueString() != "" {
		region = config.Region.ValueString()
	}

	insecure := false
	if !config.Insecure.IsNull() && !config.Insecure.IsUnknown() {
		insecure = config.Insecure.ValueBool()
	}

	client, err := newIAMClient(iamClientConfig{
		Endpoint:  config.Endpoint.ValueString(),
		Region:    region,
		AccessKey: config.AccessKey.ValueString(),
		SecretKey: config.SecretKey.ValueString(),
		Insecure:  insecure,
	})
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to configure SeaweedFS IAM client",
			fmt.Sprintf("Failed to build client: %s", err),
		)
		return
	}

	data := &providerData{
		client:    client,
		userLocks: map[string]*sync.Mutex{},
	}
	resp.ResourceData = data
	resp.DataSourceData = data
}

func (p *seaweedfsProvider) Resources(_ context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewBucketResource,
		NewIAMUserResource,
		NewIAMAccessKeyResource,
		NewIAMUserPolicyResource,
	}
}

func (p *seaweedfsProvider) DataSources(_ context.Context) []func() datasource.DataSource {
	return nil
}
