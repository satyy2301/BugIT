# BugIT DRE-Engine — PRD v2.0 (Post-Prototype)

**Status:** Phase 1 complete, Phase 2 complete, Phase 3 complete, Phase 4 complete, Phase 5 complete, Phase 6 complete  
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

## Phase 4 — IDE polish + DAP + JetBrains

| Req ID | Requirement | Status |
|--------|-------------|--------|
| REQ-IDE-001 | Remote collector snapshot load | Done — `DRE: Fetch Latest` / `Load from Collector` |
| REQ-IDE-002 | Timeline v2 + Mermaid service graph | Done — click payload, service grouping, vector graph |
| REQ-IDE-003 | Event-index DAP debugging | Done — `bugit-dre` debug adapter + replay breakpoints |
| REQ-IDE-004 | JetBrains load stub | Done — `extensions/jetbrains` tool window |

**Acceptance checklist**

- [x] Collector fetch downloads `.dre` and loads timeline (`bugit.fetchLatest`)
- [x] Mermaid graph renders from `vector_graph.edges`
- [x] Click event row shows payload detail (up to 2KB per event)
- [x] `SetBreakpoint` stops stepping at event index (`TestDebuggerBreakpointOnStep`)
- [x] VS Code **DRE: Start Replay Debug Session** launches `bugit-dre` adapter
- [x] Delve readiness poll on `dre-replay run --binary`
- [x] JetBrains plugin loads snapshot into tool window

**Verify:** `go test ./dre-replay-cli/...`, `cd extensions/vscode && npm run compile`, JetBrains `buildPlugin`

## Phase 5 — Cloud beta (EKS/GKE + observability + compliance)

| Req ID | Requirement | Status |
|--------|-------------|--------|
| REQ-CLD-001 | EKS/GKE Terraform + Helm overlays | Done — `deploy/terraform/eks`, `deploy/terraform/gke`, `values-eks.yaml`, `values-gke.yaml` |
| REQ-CLD-002 | mTLS agent→collector gRPC | Done — `pkg/grpctls`, Helm `grpc.tls`, cert-manager example |
| REQ-CLD-003 | Grafana dashboards + alerts | Done — `deploy/monitoring/` |
| REQ-CLD-004 | Cloud beta NFR gates | Done — `docs/nfr-cloud-beta.md`, `test/perf/load_test.sh` |
| REQ-CLD-005 | SOC2 mapping + runbooks | Done — `docs/compliance/`, `docs/runbooks/` |
| REQ-CLD-006 | Delve event-index correlation MVP | Done — `delve/sync.go`, VS Code status bar |

**Acceptance checklist**

- [x] Terraform provisions S3/GCS + IAM/WI for collector upload
- [x] Helm EKS/GKE values deploy with IRSA / Workload Identity annotations
- [x] Grafana JSON dashboards import for agent + collector metrics
- [x] Prometheus alert rules for bypass, drops, snapshot failures, no leader
- [x] `integration:storage` CI job with MinIO
- [x] Seek shows pid/comm in debugger TCP response and VS Code status bar
- [x] Runbooks for EKS deploy, GKE deploy, bypass, snapshot failure, rollback

**Verify:** `go test ./dre-replay-cli/...`, `cd extensions/vscode && npm run compile`, follow `docs/runbooks/eks-deploy.md`

## Phase 6 — Local plug-and-play (any OS)

| Req ID | Requirement | Status |
|--------|-------------|--------|
| REQ-LOC-001 | Unified `bugit` CLI (`capture`, `replay`, `doctor`) | Done — `bugit-cli/cmd/bugit` |
| REQ-LOC-002 | Zero-code HTTP capture on Windows/macOS/Linux | Done — `pkg/recordproxy`, `dre-collector/pkg/localcapture` |
| REQ-LOC-003 | `.bugit/` project layout + auto replay.yaml | Done — `pkg/project` |
| REQ-LOC-004 | HTTP ingest + local collector daemon | Done — `POST /v1/events`, embedded collector |
| REQ-LOC-005 | Source map + jump-to-source | Done — `source_map.json`, Node CDP, VS Code `openSourceAtEvent` |
| REQ-LOC-006 | VS Code Marketplace packaging | Done — walkthrough, bundled binary CI, `@bugit/cli` npm |
| REQ-LOC-007 | K8s path preserved as team mode | Done — `bugit capture --mode cluster` |

**Acceptance checklist**

- [ ] `npm i -g @bugit/cli` on Windows → `bugit capture` on Express app → `.dre` created
- [ ] VS Code extension loads `.bugit/latest.dre` without repo clone
- [ ] Click event opens source file at line (Node capture)
- [ ] Existing K8s capture still passes `kind_capture_test.sh`

**Verify:** `make build`, `bugit doctor`, `docs/local-quickstart.md`, `go test ./dre-collector/pkg/localcapture/...`

## Later phases

Full Go source-line ↔ event-index Delve auto-breakpoints, SOC2 Type II audit, multi-region HA buffer.
