package config

import (
	"os"
	"strconv"
)

type Config struct {
	CollectorAddr     string
	CollectorHTTPAddr string
	NodeID            string
	MetricsAddr       string
	RingbufSizeMB     int
	MockMode          bool
}

func Load() Config {
	mock := os.Getenv("DRE_MOCK_MODE") == "1"
	ringbufMB := 16
	if v := os.Getenv("DRE_RINGBUF_SIZE_MB"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			ringbufMB = n
		}
	}
	nodeID := os.Getenv("DRE_NODE_ID")
	if nodeID == "" {
		nodeID = "node-local"
	}
	return Config{
		CollectorAddr:     envOr("DRE_COLLECTOR_ADDR", "dre-collector:9090"),
		CollectorHTTPAddr: envOr("DRE_COLLECTOR_HTTP_ADDR", "http://dre-collector:8080"),
		NodeID:            nodeID,
		MetricsAddr:       envOr("DRE_METRICS_ADDR", ":9100"),
		RingbufSizeMB:     ringbufMB,
		MockMode:          mock,
	}
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
