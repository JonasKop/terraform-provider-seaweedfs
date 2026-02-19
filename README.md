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

## Generate Docs

```bash
make docs
```

This runs `tfplugindocs` via `go generate` and updates files under `docs/`.

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

- `GPG_PRIVATE_KEY`: ASCII-armored private key used to sign checksum files.
- `GPG_PASSPHRASE`: Passphrase for that key.
- `GITHUB_TOKEN` is provided automatically by GitHub Actions.

### Create First Release

```bash
git add .
git commit -m "Initial release setup"
git tag v0.1.0
git push origin main --tags
```

The release workflow will build archives and publish a GitHub release with checksums and signatures.

## Publish To Terraform Registry

1. Keep repo public and named `terraform-provider-seaweedfs`.
2. Ensure your provider address matches your namespace:
  `registry.terraform.io/jonaskop/seaweedfs`.
3. Push a semver tag (`v0.1.0`, `v0.1.1`, ...), which triggers the release workflow.
4. In Terraform Registry, publish/add the provider and link the GitHub repo `JonasKop/terraform-provider-seaweedfs`.
5. In Terraform Registry namespace settings, add your public GPG key.
6. After ingestion, users can install with:

```hcl
terraform {
  required_providers {
    seaweedfs = {
      source  = "jonaskop/seaweedfs"
      version = ">= 0.1.0"
    }
  }
}
```
