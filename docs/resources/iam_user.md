# Resource: seaweedfs_iam_user

Creates and manages a SeaweedFS IAM user.

## Example

```hcl
resource "seaweedfs_iam_user" "app" {
  name = "app-user"
  path = "/"
}
```

## Argument Reference

- `name` (required): IAM user name.
- `path` (optional): IAM path. Defaults to `/`.

## Attribute Reference

- `id`: Resource identifier (same as user name).
- `arn`: User ARN returned by the API.
- `user_id`: User ID returned by the API.
