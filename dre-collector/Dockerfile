FROM golang:1.22-bookworm AS builder
WORKDIR /src
COPY . .
RUN CGO_ENABLED=0 go build -o /out/dre-collector ./dre-collector/cmd/dre-collector
RUN CGO_ENABLED=0 go build -o /out/dre-cli ./dre-collector/cmd/dre-cli

FROM gcr.io/distroless/base-debian12
COPY --from=builder /out/dre-collector /usr/local/bin/dre-collector
COPY --from=builder /out/dre-cli /usr/local/bin/dre-cli
ENV DRE_DATA_DIR=/var/lib/dre
EXPOSE 8080 9090
ENTRYPOINT ["/usr/local/bin/dre-collector"]
