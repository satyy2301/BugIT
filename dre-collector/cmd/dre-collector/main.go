package main

import (
	"log"
	"os"

	"github.com/bugit/dre-engine/dre-collector/internal/server"
)

func main() {
	dataDir := envOr("DRE_DATA_DIR", "/var/lib/dre")
	cluster := envOr("DRE_CLUSTER", "local")
	key := envOr("DRE_SNAPSHOT_KEY", "dev-insecure-key-change-me")
	grpcAddr := envOr("DRE_GRPC_ADDR", ":9090")
	httpAddr := envOr("DRE_HTTP_ADDR", ":8080")

	collector := server.New(dataDir, cluster, key)
	if err := collector.Start(grpcAddr, httpAddr); err != nil {
		log.Fatal(err)
	}
}

func envOr(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
