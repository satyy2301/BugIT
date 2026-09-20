# BugIT DRE-Engine — PRD v2.0 (Post-Prototype)

**Status:** Phase 1 complete, Phase 2 complete, Phase 3 complete  
**Target:** Q1 2027 beta

## Phase 1 — Real capture

| Req ID | Requirement | Status |
|--------|-------------|--------|
| REQ-EBPF-001 | Socket syscall hooks capture real traffic | Done — tracepoints in `dre_probes.bpf.c` |
| REQ-EBPF-002 | clock_gettime uprobe + timeline | Done — uprobe emits `is_write=3`, snapshot prefers clock markers |
| REQ-EBPF-003 | 16MB ringbuf, drop metrics, load validation | Done — `test/perf/agent_load.sh` asserts &lt;1% drops |
| REQ-AGENT-001 | bpf2go loader on Linux | Done — `dre-agent/bpf/load.go` |
| REQ-AGENT-002 | Fail-safe bypass at memory/drop pressure | Done — `bypass_monitor.go` |

**Verify:** `make build-linux && make docker kind-load deploy-kind` then `bash test/integration/kind_capture_test.sh` and `bash test/perf/agent_load.sh`

## Phase 2 — Production collector

| Area | Status |
|------|--------|
| S3/GCS upload + presigned download | Done |
| Protobuf gRPC (no JSON codec) + health probes | Done |
| Vector clock cross-pod graph | Done |
| Auto-triggers (5xx, process-exit, SIGSEGV) | Done |
| Kernel Bearer redaction | Done |
| Leader election HA + Helm parity | Done |

## Phase 3 — Real replay + IDE + Delve

| Req ID | Requirement | Status |
|--------|-------------|--------|
| REQ-RPL-001 | Zero-network local socket stubbing | Done — multi-port proxy from `replay.yaml` |
| REQ-RPL-002 | Debugger synchronization interface | Done — Seek/GetState/RunToEvent + cursor sync |
| REQ-RPL-003 | IDE graphical control panel | Done — VS Code v2 webview controls + Delve attach launch config |

**Acceptance checklist**

- [x] Cursor stepping changes proxy output (`TestReplayCursorChangesProxyResponse`)
- [x] `replay.yaml` service ports accept stubbed traffic (`TestReplayMultiPortService`)
- [x] Debugger TCP Seek/GetState over ephemeral port (`TestDebuggerSeekOverTCP`)
- [x] Clock timeline advances on step (`TestTimeFreezeOnStep` + regenerated fixture)
- [x] VS Code panel controls + **DRE: Attach Delve** / launch.json remote attach
- [x] CI gate: `go test ./dre-replay-cli/...` + `bash test/integration/replay_e2e.sh`

**Verify:** `go test ./dre-replay-cli/...`, `bash test/integration/replay_e2e.sh`, and `dre-replay run --config deploy/replay.yaml --dre test/fixtures/demo-checkout-500.dre`

## Later phases

Phase 4+: cloud beta NFRs, full DAP breakpoint sync, JetBrains IDE.
