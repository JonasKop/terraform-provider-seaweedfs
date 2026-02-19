# Resource: seaweedfs_iam_access_key

Creates and manages an IAM access key for a SeaweedFS IAM user.

## Example

```hcl
resource "seaweedfs_iam_access_key" "app" {
  user_name = seaweedfs_iam_user.app.name
}
```

## Argument Reference

- `user_name` (required): IAM user name owning this access key.

## Attribute Reference

- `id`: Resource identifier (same as access key ID).
- `access_key_id`: Access key ID.
- `secret_access_key`: Secret access key (sensitive).
- `status`: Access key status reported by the API.
