# Encryption in transit

## Agent → collector gRPC

Production clusters should enable TLS on the ingest gRPC port (`9090`).

### Helm

Set in `values-eks.yaml` or `values-gke.yaml`:

```yaml
grpc:
  tls:
    enabled: true
    secretName: dre-grpc-tls
```

The collector mounts `tls.crt` and `tls.key`. The agent mounts `ca.crt` from the same secret (see `deploy/k8s/grpc-tls-certmanager.yaml`).

### Environment variables

| Component | Variable | Purpose |
|-----------|----------|---------|
| Collector | `DRE_GRPC_TLS_CERT_FILE` | Server certificate |
| Collector | `DRE_GRPC_TLS_KEY_FILE` | Server private key |
| Agent | `DRE_GRPC_TLS_CA_FILE` | CA bundle to verify collector |

Optional client mTLS: set `DRE_GRPC_TLS_CERT_FILE` and `DRE_GRPC_TLS_KEY_FILE` on the agent.

### cert-manager

1. Install [cert-manager](https://cert-manager.io/docs/installation/)
2. Apply `deploy/k8s/grpc-tls-certmanager.yaml`
3. Wait for `dre-grpc-tls` secret in `dre-engine` namespace
4. Helm upgrade with `grpc.tls.enabled: true`

## HTTP APIs

- Collector REST (`8080`) and metrics (`8081`) should be fronted by an ingress with TLS in production.
- Presigned S3/GCS download URLs use HTTPS with a maximum TTL of 24 hours.

## Debugger and Delve (local replay)

Replay debugger TCP (`19090`) and Delve (`2345`) are loopback-only in the default configuration. Do not expose them on public interfaces.
