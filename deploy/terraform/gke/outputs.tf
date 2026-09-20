output "gcs_bucket" {
  description = "GCS bucket name"
  value       = google_storage_bucket.snapshots.name
}

output "collector_gcp_sa_email" {
  description = "GCP service account email for Workload Identity"
  value       = google_service_account.collector.email
}

output "helm_values_snippet" {
  description = "Paste into values-gke.yaml"
  value = <<-EOT
gcs:
  bucket: ${google_storage_bucket.snapshots.name}
  prefix: dre/snapshots
collector:
  serviceAccount:
    annotations:
      iam.gke.io/gcp-service-account: ${google_service_account.collector.email}
EOT
}
