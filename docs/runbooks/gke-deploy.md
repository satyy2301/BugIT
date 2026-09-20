# Runbook: GKE deploy

## Prerequisites

- GKE cluster with Workload Identity enabled
- `kubectl`, `helm`, `terraform`, `gcloud`
- Images in Artifact Registry

## Steps

### 1. Terraform (GCS + WI)

```bash
cd deploy/terraform/gke
terraform init
terraform apply -var project_id=MY_PROJECT -var bucket_name=my-org-dre-snapshots
```

### 2. Namespace and secret

```bash
kubectl create namespace dre-engine
kubectl create secret generic dre-snapshot-key -n dre-engine \
  --from-literal=key="$(openssl rand -base64 32)"
```

### 3. Optional mTLS

```bash
kubectl apply -f deploy/k8s/grpc-tls-certmanager.yaml
```

### 4. Helm install

```bash
helm upgrade --install dre-engine ./deploy/helm/dre-engine \
  -n dre-engine \
  -f deploy/helm/dre-engine/values-gke.yaml \
  --set gcs.bucket="$(terraform -chdir=deploy/terraform/gke output -raw gcs_bucket)" \
  --set collector.serviceAccount.annotations."iam\.gke\.io/gcp-service-account"="$(terraform -chdir=deploy/terraform/gke output -raw collector_gcp_sa_email)"
```

### 5. Verify

- Agent pods on COS nodes with BTF
- Trigger snapshot; confirm object in GCS bucket
- Import Grafana dashboards from `deploy/monitoring/dashboards/`
