# DRE Engine — EKS app-layer Terraform

Provisions S3 snapshot storage and IRSA for `dre-collector`. Does **not** create the EKS cluster.

## Prerequisites

- Existing EKS cluster with OIDC provider enabled
- `kubectl` access to the cluster
- Terraform >= 1.5, AWS credentials

## Usage

```bash
cd deploy/terraform/eks
terraform init
terraform apply \
  -var cluster_name=my-eks \
  -var bucket_name=my-org-dre-snapshots \
  -var oidc_provider_arn=arn:aws:iam::123456789012:oidc-provider/oidc.eks.us-east-1.amazonaws.com/id/XXXX \
  -var oidc_provider_url=oidc.eks.us-east-1.amazonaws.com/id/XXXX
```

Copy `helm_values_snippet` from outputs into Helm, or use `values-eks.yaml` placeholders:

```bash
helm upgrade --install dre-engine ./deploy/helm/dre-engine \
  -n dre-engine --create-namespace \
  -f deploy/helm/dre-engine/values-eks.yaml \
  --set s3.bucket="$(terraform output -raw s3_bucket)" \
  --set collector.serviceAccount.annotations."eks\.amazonaws\.com/role-arn"="$(terraform output -raw collector_iam_role_arn)"
```

## Verify

1. Trigger a snapshot from the collector
2. Confirm object appears in S3 under `dre/snapshots/`
3. `kubectl logs -n dre-engine deploy/dre-collector` shows `storage_uri=s3://...`
