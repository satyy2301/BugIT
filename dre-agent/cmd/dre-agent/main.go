package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/bugit/dre-engine/dre-agent/internal/config"
	"github.com/bugit/dre-engine/dre-agent/internal/forward"
	"github.com/bugit/dre-engine/dre-agent/internal/metrics"
	"github.com/bugit/dre-engine/dre-agent/internal/probe"
	"github.com/bugit/dre-engine/api/ioevent"
)

func main() {
	cfg := config.Load()
	metrics.Register()

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	go serveMetrics(cfg.MetricsAddr)

	events := make(chan ioevent.IOEvent, 1024)
	loader := probe.NewLoader()

	if err := loader.Load(); err != nil {
		log.Printf("probe load: %v (continuing in mock/stub mode)", err)
	}

	go probe.MonitorBypass(ctx, loader)

	go func() {
		if err := loader.Run(ctx, events); err != nil {
			log.Printf("probe run: %v", err)
		}
	}()

	client := forward.New(cfg.CollectorAddr, cfg.NodeID)
	if cfg.MockMode {
		go forward.MockForwarder(ctx, cfg.CollectorAddr, cfg.NodeID, events)
	} else {
		go client.RunForwarder(ctx, events)
	}

	log.Printf("dre-agent started node=%s collector=%s", cfg.NodeID, cfg.CollectorAddr)
	<-ctx.Done()
	_ = loader.Close()
	_ = client.Close()
}

func serveMetrics(addr string) {
	mux := http.NewServeMux()
	mux.Handle("/metrics", metrics.Handler())
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	srv := &http.Server{Addr: addr, Handler: mux}
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("metrics server: %v", err)
		}
	}()
}

// emitSyntheticEvent is used in tests.
func emitSyntheticEvent(ch chan<- ioevent.IOEvent) {
	var e ioevent.IOEvent
	e.PidTgid = uint64(os.Getpid())
	e.TimestampNs = uint64(time.Now().UnixNano())
	e.Fd = 1
	e.PayloadLen = 15
	e.IsWrite = 1
	copy(e.Comm[:], []byte("dre-agent"))
	copy(e.Payload[:], []byte("HTTP/1.1 500"))
	ch <- e
	metrics.EventsEmitted.Inc()
}
