//go:build linux

package probe

import (
	"context"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/expfmt"
)

const (
	defaultMemoryLimitBytes = 128 * 1024 * 1024
	memoryBypassRatio       = 0.85
	dropBypassRatio         = 0.01
	dropWindow              = 30 * time.Second
)

// MonitorBypass enables kernel bypass when RSS or ringbuf drop rate exceeds thresholds.
func MonitorBypass(ctx context.Context, loader *Loader) {
	if os.Getenv("DRE_SKIP_BPF") == "1" {
		return
	}

	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	var lastEmitted float64
	var lastDrops float64
	windowStart := time.Now()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if loader.Bypassed() {
				continue
			}

			limit := cgroupMemoryLimit()
			rss := processRSS()
			if limit > 0 && rss > uint64(float64(limit)*memoryBypassRatio) {
				log.Printf("memory pressure: rss=%d limit=%d, enabling bypass", rss, limit)
				loader.SetBypass(true)
				continue
			}

			emitted, drops := readAgentMetrics()
			deltaEmitted := emitted - lastEmitted
			deltaDrops := drops - lastDrops
			if time.Since(windowStart) >= dropWindow {
				if deltaEmitted+deltaDrops > 0 {
					rate := deltaDrops / (deltaEmitted + deltaDrops)
					if rate > dropBypassRatio {
						log.Printf("ringbuf drop rate %.2f%% exceeds threshold, enabling bypass", rate*100)
						loader.SetBypass(true)
					}
				}
				windowStart = time.Now()
				lastEmitted = emitted
				lastDrops = drops
			}
		}
	}
}

func cgroupMemoryLimit() uint64 {
	if v := os.Getenv("DRE_MEMORY_LIMIT_BYTES"); v != "" {
		if n, err := strconv.ParseUint(v, 10, 64); err == nil && n > 0 {
			return n
		}
	}
	paths := []string{
		"/sys/fs/cgroup/memory.max",
		"/sys/fs/cgroup/memory/memory.limit_in_bytes",
	}
	for _, p := range paths {
		b, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		s := strings.TrimSpace(string(b))
		if s == "max" || s == "" {
			continue
		}
		v, err := strconv.ParseUint(s, 10, 64)
		if err != nil || v == 0 {
			continue
		}
		return v
	}
	return defaultMemoryLimitBytes
}

func processRSS() uint64 {
	b, err := os.ReadFile("/proc/self/status")
	if err != nil {
		return 0
	}
	for _, line := range strings.Split(string(b), "\n") {
		if strings.HasPrefix(line, "VmRSS:") {
			fields := strings.Fields(line)
			if len(fields) < 2 {
				return 0
			}
			kb, err := strconv.ParseUint(fields[1], 10, 64)
			if err != nil {
				return 0
			}
			return kb * 1024
		}
	}
	return 0
}

func readAgentMetrics() (emitted float64, drops float64) {
	addr := os.Getenv("DRE_METRICS_ADDR")
	if addr == "" {
		addr = ":9100"
	}
	if !strings.HasPrefix(addr, "http") {
		addr = "http://127.0.0.1" + addr
	}

	resp, err := http.Get(addr + "/metrics")
	if err != nil {
		return 0, 0
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, 0
	}

	var parser expfmt.TextParser
	metricFamilies, err := parser.TextToMetricFamilies(strings.NewReader(string(body)))
	if err != nil {
		return 0, 0
	}
	return counterValue(metricFamilies, "dre_events_emitted_total"),
		counterValue(metricFamilies, "dre_ringbuf_drops_total")
}

func counterValue(families map[string]*dto.MetricFamily, name string) float64 {
	mf := families[name]
	if mf == nil || len(mf.Metric) == 0 || mf.Metric[0].Counter == nil {
		return 0
	}
	return mf.Metric[0].Counter.GetValue()
}
