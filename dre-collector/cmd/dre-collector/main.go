package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"

	"github.com/bugit/dre-engine/dre-collector/internal/leader"
	"github.com/bugit/dre-engine/dre-collector/internal/metrics"
	"github.com/bugit/dre-engine/dre-collector/internal/server"
	"github.com/bugit/dre-engine/dre-collector/internal/storage"
)

func main() {
	dataDir := envOr("DRE_DATA_DIR", "/var/lib/dre")
	cluster := envOr("DRE_CLUSTER", "local")
	key := envOr("DRE_SNAPSHOT_KEY", "dev-insecure-key-change-me")
	grpcAddr := envOr("DRE_GRPC_ADDR", ":9090")
	httpAddr := envOr("DRE_HTTP_ADDR", ":8080")

	cfg := storage.ConfigFromEnv()
	uploader, err := storage.NewFromConfig(cfg)
	if err != nil {
		log.Fatalf("storage: %v", err)
	}
	if uploader != nil {
		log.Printf("object storage enabled (s3=%s gcs=%s)", cfg.S3Bucket, cfg.GCSBucket)
	}

	metrics.Register()
	var isLeader atomic.Bool
	metricsAddr := envOr("DRE_METRICS_ADDR", ":8081")
	go serveMetricsAndReadiness(metricsAddr, &isLeader)

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	collector := server.New(dataDir, cluster, key, uploader)
	if err := leader.RunElection(ctx, func(leadCtx context.Context) {
		isLeader.Store(true)
		defer isLeader.Store(false)
		if err := collector.Run(leadCtx, grpcAddr, httpAddr); err != nil {
			log.Printf("collector stopped: %v", err)
		}
	}); err != nil {
		log.Fatal(err)
	}
}

func serveMetricsAndReadiness(addr string, isLeader *atomic.Bool) {
	mux := http.NewServeMux()
	mux.Handle("/metrics", metrics.Handler())
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	mux.HandleFunc("/readyz", func(w http.ResponseWriter, _ *http.Request) {
		if os.Getenv("DRE_LEADER_ELECT") == "1" && !isLeader.Load() {
			http.Error(w, "not leader", http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ready"))
	})
	_ = http.ListenAndServe(addr, mux)
}

func envOr(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
