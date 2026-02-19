variable "endpoint" {
  type        = string
  description = "SeaweedFS endpoint."
}

variable "region" {
  type        = string
  description = "SigV4 region."
  default     = "us-east-1"
}

variable "access_key" {
  type        = string
  description = "Admin access key."
  sensitive   = true
}

variable "secret_key" {
  type        = string
  description = "Admin secret key."
  sensitive   = true
}

variable "insecure" {
  type        = bool
  description = "Skip TLS verification."
  default     = false
}

variable "user_name" {
  type        = string
  description = "User to create."
  default     = "tf-seaweedfs-test-user"
}

variable "bucket_name" {
  type        = string
  description = "Bucket to create for testing."
  default     = "tf-seaweedfs-test-bucket"
}

variable "policy_name" {
  type        = string
  description = "Inline policy name for the test user."
  default     = "tf-seaweedfs-test-policy"
}

variable "create_bucket" {
  type        = bool
  description = "Whether to create the test bucket."
  default     = true
}

variable "create_user_policy" {
  type        = bool
  description = "Whether to create and attach a user inline policy."
  default     = true
}

variable "create_access_key" {
  type        = bool
  description = "Whether to create an IAM access key for the user."
  default     = false
}
