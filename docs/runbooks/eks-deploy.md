# Runbook: EKS deploy

## Prerequisites

- EKS cluster with OIDC provider
- `kubectl`, `helm`, `terraform`, AWS CLI
- Container images pushed to ECR

## Steps

### 1. Terraform (storage + IRSA)

```bash
cd deploy/terraform/eks
terraform init
terraform apply \
  -var cluster_name=MY_CLUSTER \
  -var bucket_name=my-org-dre-snapshots \
  -var oidc_provider_arn=arn:aws:iam::ACCOUNT:oidc-provider/oidc.eks.REGION.amazonaws.com/id/ID \
  -var oidc_provider_url=oidc.eks.REGION.amazonaws.com/id/ID
```

### 2. Namespace and encryption secret

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
  -f deploy/helm/dre-engine/values-eks.yaml \
  --set s3.bucket="$(terraform -chdir=deploy/terraform/eks output -raw s3_bucket)" \
  --set collector.serviceAccount.annotations."eks\.amazonaws\.com/role-arn"="$(terraform -chdir=deploy/terraform/eks output -raw collector_iam_role_arn)"
```

### 5. Verify capture

```bash
kubectl port-forward -n dre-engine svc/dre-collector 8080:8080
curl -X POST http://localhost:8080/v1/trigger
aws s3 ls s3://BUCKET/dre/snapshots/
```

See also [`docs/demo-walkthrough.md`](../demo-walkthrough.md) for VS Code load steps.
