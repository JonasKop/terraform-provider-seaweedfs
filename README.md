# terraform-provider-seaweedfs

Custom Terraform provider for SeaweedFS IAM operations.

## Current scope

- `seaweedfs_bucket`
  - Create via S3 `PUT /{bucket}`
  - Read via S3 `HEAD /{bucket}`
  - Delete via S3 `DELETE /{bucket}`
- `seaweedfs_iam_user`
  - Create via `CreateUser`
  - Read via `GetUser`
  - Delete via `DeleteUser`
- `seaweedfs_iam_access_key`
  - Create via `CreateAccessKey`
  - Read via `ListAccessKeys`
  - Delete via `DeleteAccessKey`
- `seaweedfs_iam_user_policy`
  - Create/Update via `PutUserPolicy`
  - Read via `GetUserPolicy`
  - Delete via `DeleteUserPolicy`

The provider intentionally avoids IAM actions that are commonly unsupported by SeaweedFS compatibility layers (for example group-membership listing during user deletion).

## Observed SeaweedFS behavior

- In live tests against a SeaweedFS S3 endpoint, user, user policy, and bucket CRUD worked.
- `CreateAccessKey` can return `ServiceFailure: Internal server error` in some SeaweedFS deployments.

## Build

```bash
go mod tidy
go build -o terraform-provider-seaweedfs
```

## Use locally with Terraform

Create `~/.terraformrc`:

```hcl
provider_installation {
  dev_overrides {
    "jonaskop/seaweedfs" = "/absolute/path/to/terraform-provider-seaweedfs"
  }
  direct {}
}
```

Then in your Terraform config:

```hcl
terraform {
  required_providers {
    seaweedfs = {
      source = "jonaskop/seaweedfs"
    }
  }
}

provider "seaweedfs" {
  endpoint   = "https://s3.example.com"
  region     = "us-east-1"
  access_key = var.admin_access_key
  secret_key = var.admin_secret_key
}

resource "seaweedfs_iam_user" "example" {
  name = "example-user"
}
```

## CI and Release

- CI workflow: `.github/workflows/ci.yml`
- Release workflow: `.github/workflows/release.yml`
- GoReleaser config: `.goreleaser.yml`

### Required GitHub Secrets for Release

- No extra signing secrets are required for the current workflow.
- `GITHUB_TOKEN` is provided automatically by GitHub Actions.

### Create First Release

```bash
git add .
git commit -m "Initial release setup"
git tag v0.1.0
git push origin main --tags
```

The release workflow will build archives and publish a GitHub release with checksums and signatures.
