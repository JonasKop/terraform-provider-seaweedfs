# Changelog

All notable changes to this project will be documented in this file.

The format is based on Keep a Changelog and this project adheres to Semantic Versioning.

## [0.1.1] - 2026-02-19

### Fixed

- Eliminated recurring `seaweedfs_iam_user_policy` drift by treating policy content as write-only for drift comparison during `Read` while still verifying remote policy existence.
- Improved IAM convergence and idempotency for SeaweedFS eventual consistency:
  - Added retry/backoff for transient IAM errors (`NoSuchEntity`, `ServiceFailure`, `HTTP500`, `HTTP503`).
  - Made user creation idempotent by adopting existing users on `EntityAlreadyExists`.
  - Added post-create user visibility checks before finishing `seaweedfs_iam_user` creation.
  - Serialized mutating IAM operations globally to reduce cross-user write race failures.
- Made bucket creation idempotent by handling already-existing buckets and verifying with `HeadBucket`.

## [0.1.0] - 2026-02-19

### Added

- Initial provider implementation using Terraform Plugin Framework.
- Resources:
  - `seaweedfs_bucket`
  - `seaweedfs_iam_user`
  - `seaweedfs_iam_access_key`
  - `seaweedfs_iam_user_policy`
- Signed AWS SigV4 request client for IAM and S3-compatible calls.
- Per-user operation locking to reduce concurrency issues for mutating IAM calls.
- Basic example configuration in `examples/basic`.
