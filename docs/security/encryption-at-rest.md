# Encryption at rest

## Snapshot format (`.dre`)

Incident snapshots are tar.gz archives encrypted with **AES-256-GCM**. The encryption key is supplied via:

- Kubernetes secret `dre-snapshot-key` (`DRE_SNAPSHOT_KEY` env)
- Local replay: `--key` flag or `DRE_SNAPSHOT_KEY`

Rotate keys by re-exporting snapshots with a new key; old snapshots remain readable only with the original key.

## Object storage

### AWS S3 (EKS)

Terraform module `deploy/terraform/eks` configures:

- Bucket default encryption: SSE-KMS
- IRSA role scoped to `PutObject`, `GetObject`, `ListBucket` on the snapshot prefix

### GCS (GKE)

Terraform module `deploy/terraform/gke` configures:

- Uniform bucket-level access
- Optional CMEK via `kms_key_name` variable

## PVC data

Collector rolling-buffer data on PVC is ephemeral working state. Durable incident data is the encrypted `.dre` object in object storage.

## Key rotation

1. Create new KMS key (AWS) or CMEK version (GCP)
2. Update Terraform / bucket default encryption
3. Rotate `DRE_SNAPSHOT_KEY` in Kubernetes secrets
4. Re-trigger export for active incidents if needed
