# Runbook: Upgrade and rollback

## Helm upgrade

```bash
helm upgrade dre-engine ./deploy/helm/dre-engine \
  -n dre-engine \
  -f deploy/helm/dre-engine/values-eks.yaml
```

Review diff first:

```bash
helm diff upgrade dre-engine ./deploy/helm/dre-engine -n dre-engine -f values-eks.yaml
```

## Rollback

```bash
helm history dre-engine -n dre-engine
helm rollback dre-engine REVISION -n dre-engine
```

## Snapshot compatibility

- `.dre` format is backward compatible within beta (manifest v1)
- Encryption key must remain unchanged across rollback unless re-exporting snapshots
- Replay CLI version should match or exceed snapshot generator version

## Agent DaemonSet notes

- Rolling update may briefly duplicate agents per node; expect short metric gaps
- BPF programs reload on pod restart; verify `dre_bypass_mode` after upgrade

## Post-upgrade checks

1. `kubectl get pods -n dre-engine`
2. `curl http://localhost:8081/readyz` (metrics port on collector)
3. Trigger test snapshot
4. `go test ./dre-replay-cli/...` with latest fixture
