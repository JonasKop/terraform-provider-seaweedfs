terraform {
  required_providers {
    seaweedfs = {
      source = "jonaskop/seaweedfs"
    }
  }
}

provider "seaweedfs" {
  endpoint   = var.endpoint
  region     = var.region
  access_key = var.access_key
  secret_key = var.secret_key
  insecure   = var.insecure
}

resource "seaweedfs_iam_user" "test" {
  name = var.user_name
}

resource "seaweedfs_bucket" "test" {
  count  = var.create_bucket ? 1 : 0
  bucket = var.bucket_name
}

resource "seaweedfs_iam_user_policy" "test" {
  count     = var.create_user_policy ? 1 : 0
  user_name = seaweedfs_iam_user.test.name
  name      = var.policy_name
  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Effect = "Allow"
      Action = ["s3:*"]
      Resource = [
        "arn:aws:s3:::${var.bucket_name}",
        "arn:aws:s3:::${var.bucket_name}/*",
      ]
    }]
  })
}

resource "seaweedfs_iam_access_key" "test" {
  count     = var.create_access_key ? 1 : 0
  user_name = seaweedfs_iam_user.test.name
}

output "user_name" {
  value = seaweedfs_iam_user.test.name
}

output "user_arn" {
  value = seaweedfs_iam_user.test.arn
}

output "bucket_name" {
  value = var.create_bucket ? seaweedfs_bucket.test[0].bucket : null
}

output "access_key_id" {
  value     = var.create_access_key ? seaweedfs_iam_access_key.test[0].access_key_id : null
  sensitive = true
}
