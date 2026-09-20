# Development Environments

## Recommended: Native Linux + kind (Beta)

1. Run `bash scripts/setup-dev.sh`
2. Run `bash scripts/kind-up.sh`
3. `make bpf && make build && make docker && make deploy-kind`

## Cloud Kubernetes (Production)

Deploy via Helm:

```bash
helm install dre-engine deploy/helm/dre-engine \
  --set snapshot.encryptionSecret=dre-snapshot-key \
  --set s3.bucket=my-dre-snapshots
```

## Windows + WSL2

eBPF builds must run inside WSL2 with a BTF-enabled kernel:

```bash
# Inside WSL2 Ubuntu
cd /mnt/c/Users/.../BugIT
bash scripts/setup-dev.sh
make bpf build
```

Replay CLI and VS Code extension work on Windows natively.
