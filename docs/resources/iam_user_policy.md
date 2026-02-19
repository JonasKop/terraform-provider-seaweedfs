# Resource: seaweedfs_iam_user_policy

Creates and manages an inline IAM policy attached to a SeaweedFS IAM user.

## Example

```hcl
resource "seaweedfs_iam_user_policy" "app" {
  user_name = seaweedfs_iam_user.app.name
  name      = "app-policy"
  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Effect   = "Allow"
      Action   = ["s3:*"]
      Resource = ["arn:aws:s3:::app-bucket", "arn:aws:s3:::app-bucket/*"]
    }]
  })
}
```

## Argument Reference

- `user_name` (required): IAM user name.
- `name` (required): Policy name.
- `policy` (required): Policy JSON document string.

## Attribute Reference

- `id`: Resource identifier in the form `<user_name>:<name>`.
