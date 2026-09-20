# BugIT DRE-Engine — PRD v2.0 (Post-Prototype)

**Status:** Phase 1 in progress  
**Target:** Q1 2027 beta

## Phase 1 — Real capture (current)

| Req ID | Requirement | Status |
|--------|-------------|--------|
| REQ-EBPF-001 | Socket syscall hooks capture real traffic | Implemented — tracepoints in `dre_probes.bpf.c` |
| REQ-EBPF-003 | 16MB ringbuf, drop metrics | Implemented — `dre_ringbuf_drops_total` |
| REQ-AGENT-001 | bpf2go loader on Linux | Implemented — `dre-agent/bpf/load.go` |
| REQ-AGENT-002 | Fail-safe bypass at memory/drop pressure | Implemented — `bypass_monitor.go` |
| REQ-EBPF-002 | clock_gettime uprobe | Deferred — shim exists, kernel capture TBD |

**Verify:** `make build-linux && make docker kind-load deploy-kind` then `bash test/integration/kind_capture_test.sh`

## Later phases

See project plan: production collector (S3, protobuf), real replay fidelity, IDE + Delve, cloud beta NFRs.
