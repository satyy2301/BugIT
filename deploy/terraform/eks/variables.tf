variable "cluster_name" {
  description = "EKS cluster name (must exist; used for IRSA trust)"
  type        = string
}

variable "namespace" {
  description = "Kubernetes namespace for DRE workloads"
  type        = string
  default     = "dre-engine"
}

variable "bucket_name" {
  description = "S3 bucket name for encrypted .dre snapshots"
  type        = string
}

variable "kms_key_arn" {
  description = "Optional KMS key ARN for bucket encryption (creates key if empty)"
  type        = string
  default     = ""
}

variable "oidc_provider_arn" {
  description = "EKS OIDC provider ARN for IRSA"
  type        = string
}

variable "oidc_provider_url" {
  description = "EKS OIDC issuer URL without https:// prefix"
  type        = string
}

variable "tags" {
  description = "Tags applied to AWS resources"
  type        = map(string)
  default     = {}
}
