# SeaweedFS Provider

The `seaweedfs` provider manages IAM users, IAM access keys, IAM inline user policies, and S3 buckets for SeaweedFS deployments exposing AWS-compatible APIs.

## Provider Configuration

```hcl
provider "seaweedfs" {
  endpoint   = "https://s3.example.com"
  region     = "us-east-1"
  access_key = var.admin_access_key
  secret_key = var.admin_secret_key
  insecure   = false
}
```

### Arguments

- `endpoint` (required): SeaweedFS endpoint URL.
- `region` (optional): SigV4 signing region. Defaults to `us-east-1`.
- `access_key` (required, sensitive): Admin key used for API calls.
- `secret_key` (required, sensitive): Admin secret used for API calls.
- `insecure` (optional): Skip TLS verification when true.
