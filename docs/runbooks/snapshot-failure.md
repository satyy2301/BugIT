# Runbook: Snapshot export failure

## Symptoms

- Alert **DreCollectorSnapshotFailures**
- `dre-cli trigger` returns error or no new snapshot in list
- Logs: `snapshot export failed` or `snapshot upload failed`

## Diagnosis

1. Leader pod: `kubectl get lease -n dre-engine` — only leader exports
2. Collector logs: `kubectl logs -n dre-engine deploy/dre-collector --tail=200`
3. PVC space: `kubectl exec -n dre-engine deploy/dre-collector -- df -h /var/lib/dre`
4. S3/GCS: verify `DRE_S3_BUCKET` or `DRE_GCS_BUCKET` and IAM/WI bindings

## Remediation

| Cause | Fix |
|-------|-----|
| Not leader | Wait for election; ensure 2+ replicas if using HA |
| Disk full | Expand PVC or prune old local snapshots |
| Upload denied | Fix IRSA role (EKS) or Workload Identity (GKE) per terraform README |
| Encryption key | Verify `dre-snapshot-key` secret matches agent/collector config |

## Verify

```bash
curl -X POST http://localhost:8080/v1/trigger
curl http://localhost:8080/v1/snapshots
```

Or `dre-cli trigger --collector http://localhost:8080`
