FROM golang:1.22-bookworm AS builder
RUN apt-get update && apt-get install -y clang llvm libbpf-dev linux-headers-amd64
WORKDIR /src
COPY . .
RUN make bpf build

FROM debian:bookworm-slim
RUN apt-get update && apt-get install -y libbpf1 ca-certificates && rm -rf /var/lib/apt/lists/*
COPY --from=builder /src/bin/dre-agent /usr/local/bin/dre-agent
ENV DRE_COLLECTOR_ADDR=dre-collector:9090
EXPOSE 9100
ENTRYPOINT ["/usr/local/bin/dre-agent"]
