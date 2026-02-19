# Resource: seaweedfs_bucket

Creates and manages a SeaweedFS bucket using S3-compatible API calls.

## Example

```hcl
resource "seaweedfs_bucket" "logs" {
  bucket = "logs-bucket"
}
```

## Argument Reference

- `bucket` (required): Bucket name.

## Attribute Reference

- `id`: Resource identifier (same as bucket name).
- `arn`: Bucket ARN in the form `arn:aws:s3:::<bucket>`.
