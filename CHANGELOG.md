# Changelog

All notable changes to this project will be documented in this file.

The format is based on Keep a Changelog and this project adheres to Semantic Versioning.

## [Unreleased]

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
