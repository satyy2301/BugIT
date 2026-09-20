variable "project_id" {
  description = "GCP project ID"
  type        = string
}

variable "region" {
  description = "GCS bucket region"
  type        = string
  default     = "us-central1"
}

variable "bucket_name" {
  description = "GCS bucket name for encrypted .dre snapshots"
  type        = string
}

variable "namespace" {
  description = "Kubernetes namespace for DRE workloads"
  type        = string
  default     = "dre-engine"
}

variable "collector_ksa_name" {
  description = "Collector Kubernetes service account name"
  type        = string
  default     = "dre-collector"
}

variable "gke_sa_id" {
  description = "GCP service account ID for collector (without @project)"
  type        = string
  default     = "dre-collector"
}

variable "labels" {
  description = "Labels applied to GCS bucket"
  type        = map(string)
  default     = {}
}

variable "kms_key_name" {
  description = "Optional Cloud KMS key for bucket encryption"
  type        = string
  default     = ""
}
