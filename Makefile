.PHONY: all bpf build build-linux test lint docker kind-up kind-load deploy-kind fetch-snapshot clean proto demo-snapshot

GO       ?= go
CLANG    ?= clang
LLVM_STRIP ?= llvm-strip
BPF2GO   ?= go run github.com/cilium/ebpf/cmd/bpf2go@v0.16.0

BIN_DIR  := bin
AGENT_BIN := $(BIN_DIR)/dre-agent
COLLECTOR_BIN := $(BIN_DIR)/dre-collector
CLI_BIN := $(BIN_DIR)/dre-cli
REPLAY_BIN := $(BIN_DIR)/dre-replay

AGENT_IMAGE := bugit/dre-agent:dev
COLLECTOR_IMAGE := bugit/dre-collector:dev

all: build

VMLINUX_HDR := dre-agent/bpf/vmlinux.h

bpf: $(VMLINUX_HDR)
	@if [ "$(OS)" = "Windows_NT" ] 2>/dev/null || [ "$$OS" = "Windows_NT" ]; then \
		echo "Skipping bpf build on Windows (run on Linux or WSL2)"; \
	else \
		$(BPF2GO) -cc $(CLANG) -cflags "-O2 -g -Wall" -target amd64,arm64 \
			-go-package bpf -output-dir dre-agent/bpf \
			bpf dre-agent/bpf/dre_probes.bpf.c -- -I./api -I./dre-agent/bpf; \
	fi

$(VMLINUX_HDR):
	@if [ "$(OS)" = "Windows_NT" ] 2>/dev/null || [ "$$OS" = "Windows_NT" ]; then \
		echo "Skipping vmlinux.h generation on Windows"; \
	elif [ ! -f "$(VMLINUX_HDR)" ]; then \
		bash scripts/gen-vmlinux.sh "$(VMLINUX_HDR)"; \
	fi

build-linux: bpf build

proto:
	$(GO) install google.golang.org/protobuf/cmd/protoc-gen-go@latest
	$(GO) install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
	protoc --go_out=. --go_opt=paths=source_relative \
		--go-grpc_out=. --go-grpc_opt=paths=source_relative \
		api/proto/dre/v1/events.proto

build:
	@$(GO) build -o $(AGENT_BIN) ./dre-agent/cmd/dre-agent
	@$(GO) build -o $(COLLECTOR_BIN) ./dre-collector/cmd/dre-collector
	@$(GO) build -o $(CLI_BIN) ./dre-collector/cmd/dre-cli
	@$(GO) build -o $(REPLAY_BIN) ./dre-replay-cli/cmd/dre-replay

demo-snapshot:
	$(GO) run ./scripts/generate-demo-snapshot -out test/fixtures/demo-checkout-500.dre

test:
	$(GO) test ./...

lint:
	@which golangci-lint >/dev/null 2>&1 && golangci-lint run ./... || echo "golangci-lint not installed, skipping"

docker:
	docker build -t $(AGENT_IMAGE) -f dre-agent/Dockerfile .
	docker build -t $(COLLECTOR_IMAGE) -f dre-collector/Dockerfile .

kind-up:
	bash scripts/kind-up.sh

kind-load:
	kind load docker-image $(AGENT_IMAGE) --name dre-engine
	kind load docker-image $(COLLECTOR_IMAGE) --name dre-engine

deploy-kind:
	kubectl apply -f deploy/k8s/namespace.yaml
	kubectl apply -f deploy/k8s/dre-collector.yaml
	kubectl apply -f deploy/k8s/dre-agent.yaml
	kubectl apply -f deploy/k8s/nginx-sample.yaml
	kubectl rollout status deployment/dre-collector -n dre-engine --timeout=120s
	kubectl rollout status deployment/nginx-sample -n dre-engine --timeout=120s

fetch-snapshot:
	@bash scripts/fetch-snapshot.sh

clock-shim:
	$(CLANG) -shared -fPIC -O2 -o $(BIN_DIR)/clock_shim.so dre-replay-cli/shim/clock_shim.c -ldl

clean:
	rm -rf $(BIN_DIR) dre-agent/bpf/bpf_*.go dre-agent/bpf/bpf_*.o
