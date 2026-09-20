terraform {
  required_version = ">= 1.5"
  required_providers {
    google = {
      source  = "hashicorp/google"
      version = ">= 5.0"
    }
  }
}

provider "google" {
  project = var.project_id
  region  = var.region
}

resource "google_storage_bucket" "snapshots" {
  name                        = var.bucket_name
  location                    = var.region
  uniform_bucket_level_access = true
  force_destroy               = false
  labels                      = var.labels

  encryption {
    default_kms_key_name = var.kms_key_name != "" ? var.kms_key_name : null
  }
}

resource "google_service_account" "collector" {
  account_id   = var.gke_sa_id
  display_name = "DRE collector snapshot uploader"
}

resource "google_storage_bucket_iam_member" "collector_writer" {
  bucket = google_storage_bucket.snapshots.name
  role   = "roles/storage.objectAdmin"
  member = "serviceAccount:${google_service_account.collector.email}"
}

resource "google_service_account_iam_member" "wi_binding" {
  service_account_id = google_service_account.collector.name
  role               = "roles/iam.workloadIdentityUser"
  member             = "serviceAccount:${var.project_id}.svc.id.goog[${var.namespace}/${var.collector_ksa_name}]"
}
