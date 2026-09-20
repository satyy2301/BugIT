# DRE Engine — GKE app-layer Terraform

Provisions GCS snapshot storage and Workload Identity for `dre-collector`. Does **not** create the GKE cluster.

## Prerequisites

- Existing GKE cluster with Workload Identity enabled
- `kubectl` access
- Terraform >= 1.5, `gcloud` authenticated

## Usage

```bash
cd deploy/terraform/gke
terraform init
terraform apply \
  -var project_id=my-gcp-project \
  -var bucket_name=my-org-dre-snapshots \
  -var region=us-central1
```

Deploy with Helm:

```bash
helm upgrade --install dre-engine ./deploy/helm/dre-engine \
  -n dre-engine --create-namespace \
  -f deploy/helm/dre-engine/values-gke.yaml \
  --set gcs.bucket="$(terraform output -raw gcs_bucket)" \
  --set collector.serviceAccount.annotations."iam\.gke\.io/gcp-service-account"="$(terraform output -raw collector_gcp_sa_email)"
```

## Notes

- Agent DaemonSet requires COS nodes with BTF (`values-gke.yaml` nodeSelector).
- Annotate the collector ServiceAccount with Workload Identity before pods start.
