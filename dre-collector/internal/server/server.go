package server

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/bugit/dre-engine/api/grpcapi"
	"github.com/bugit/dre-engine/api/ioevent"
	"github.com/bugit/dre-engine/api/manifest"
	"github.com/bugit/dre-engine/dre-collector/internal/buffer"
	"github.com/bugit/dre-engine/dre-collector/internal/ingest"
	"github.com/bugit/dre-engine/dre-collector/internal/metrics"
	"github.com/bugit/dre-engine/dre-collector/internal/redact"
	"github.com/bugit/dre-engine/dre-collector/internal/snapshot"
	"github.com/bugit/dre-engine/dre-collector/internal/storage"
	"github.com/bugit/dre-engine/dre-collector/internal/trigger"
	"github.com/bugit/dre-engine/dre-collector/internal/vector"
	"github.com/bugit/dre-engine/pkg/grpctls"
	"github.com/bugit/dre-engine/pkg/sourcemap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
)

type LocalHooks struct {
	IncidentBuilder func(events []ioevent.IOEvent) *manifest.Incident
	SourceMap       *sourcemap.Store
	ReplayWriter    func(path, root string, events []ioevent.IOEvent) error
	SnapshotDir     string
	LatestLink      string
	ReplayPath      string
	ProjectRoot     string
}

type Collector struct {
	grpcapi.UnimplementedEventIngestServer
	grpcapi.UnimplementedCollectorAdminServer

	buf      *buffer.RollingBuffer
	vector   *vector.Engine
	exporter *snapshot.Exporter
	ingest   *ingest.Service
	triggers *trigger.Engine
	storage  storage.Uploader
	httpBase string
	local    LocalHooks

	mu        sync.RWMutex
	snapshots []*grpcapi.SnapshotInfo
	ready     atomic.Bool
}

func New(dataDir, cluster, key string, uploader storage.Uploader) *Collector {
	buf := buffer.New()
	vec := vector.New()
	c := &Collector{
		buf:      buf,
		vector:   vec,
		exporter: snapshot.New(dataDir, cluster, key),
		storage:  uploader,
		httpBase: envOr("DRE_HTTP_PUBLIC_URL", ""),
	}
	c.triggers = trigger.New(c.captureSnapshot)
	if rules, err := trigger.LoadRules(envOr("DRE_TRIGGER_RULES_PATH", "/etc/dre/trigger_rules.yaml")); err != nil {
		log.Printf("trigger rules: %v (using defaults)", err)
	} else {
		c.triggers.SetRules5xx(rules)
	}
	c.ingest = ingest.New(buf, c.triggers, vec)
	return c
}

func (c *Collector) SetLocalHooks(h LocalHooks) {
	c.local = h
	if c.exporter != nil {
		c.exporter.SetSourceMap(h.SourceMap)
	}
}

func (c *Collector) captureSnapshot(trig manifest.Trigger) {
	start := time.Now()
	events := c.buf.Snapshot()
	if c.local.IncidentBuilder != nil {
		if inc := c.local.IncidentBuilder(bufferEventsToNative(events)); inc != nil {
			c.exporter.SetIncident(inc)
		}
	}
	graph := c.vector.Graph()
	var redactLog manifest.RedactionLog
	for i := range events {
		scrubbed, log := redact.Scrub(events[i].IOEvent.Payload[:events[i].IOEvent.PayloadLen])
		events[i].IOEvent.PayloadLen = uint32(len(scrubbed))
		copy(events[i].IOEvent.Payload[:], scrubbed)
		redactLog.Entries = append(redactLog.Entries, log.Entries...)
	}
	m, path, err := c.exporter.Export(events, graph, redactLog, trig)
	if err != nil {
		metrics.SnapshotExportFailures.Inc()
		log.Printf("snapshot export failed: %v", err)
		return
	}

	info := &grpcapi.SnapshotInfo{
		Id:         m.ID,
		Path:       path,
		CapturedAt: m.CapturedAt.Format(timeRFC3339),
		EventCount: int64(m.EventCount),
	}

	if c.storage != nil {
		data, err := os.ReadFile(path)
		if err != nil {
			log.Printf("snapshot read for upload: %v", err)
		} else {
			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()
			uri, err := c.storage.Upload(ctx, m.ID, data)
			if err != nil {
				log.Printf("snapshot upload failed: %v", err)
			} else {
				info.StorageUri = uri
				if url, err := c.storage.PresignGet(ctx, m.ID, 24*time.Hour); err == nil {
					info.DownloadUrl = url
				}
			}
		}
	}
	if info.DownloadUrl == "" && c.httpBase != "" {
		info.DownloadUrl = strings.TrimRight(c.httpBase, "/") + "/v1/snapshots/" + m.ID + "/download"
	}

	c.mu.Lock()
	c.snapshots = append(c.snapshots, info)
	c.mu.Unlock()
	metrics.SnapshotsExported.Inc()
	metrics.SnapshotExportDuration.Observe(time.Since(start).Seconds())
	log.Printf("snapshot written: %s events=%d storage=%s", path, m.EventCount, info.StorageUri)
	c.postProcessSnapshot(path, events)
}

func (c *Collector) postProcessSnapshot(path string, events []buffer.Event) {
	if c.local.SnapshotDir == "" {
		return
	}
	_ = os.MkdirAll(c.local.SnapshotDir, 0o755)
	base := filepath.Base(path)
	dest := filepath.Join(c.local.SnapshotDir, base)
	if path != dest {
		data, err := os.ReadFile(path)
		if err == nil {
			_ = os.WriteFile(dest, data, 0o644)
			path = dest
		}
	}
	if c.local.LatestLink != "" {
		data, err := os.ReadFile(path)
		if err == nil {
			_ = os.WriteFile(c.local.LatestLink, data, 0o644)
		}
	}
	if c.local.ReplayPath != "" && len(events) > 0 && c.local.ReplayWriter != nil {
		if err := c.local.ReplayWriter(c.local.ReplayPath, c.local.ProjectRoot, bufferEventsToNative(events)); err != nil {
			log.Printf("replay config: %v", err)
		}
	}
}

func bufferEventsToNative(events []buffer.Event) []ioevent.IOEvent {
	out := make([]ioevent.IOEvent, len(events))
	for i := range events {
		out[i] = events[i].IOEvent
	}
	return out
}
const timeRFC3339 = "2006-01-02T15:04:05Z"

func (c *Collector) StreamEvents(stream grpcapi.EventIngest_StreamEventsServer) error {
	return c.ingest.StreamEvents(stream)
}

func (c *Collector) TriggerSnapshot(_ context.Context, req *grpcapi.TriggerSnapshotRequest) (*grpcapi.TriggerSnapshotResponse, error) {
	c.triggers.Manual(req.Detail)
	c.mu.RLock()
	defer c.mu.RUnlock()
	if len(c.snapshots) == 0 {
		return &grpcapi.TriggerSnapshotResponse{SnapshotId: "pending", Path: ""}, nil
	}
	last := c.snapshots[len(c.snapshots)-1]
	return &grpcapi.TriggerSnapshotResponse{
		SnapshotId:  last.Id,
		Path:        last.Path,
		StorageUri:  last.StorageUri,
		DownloadUrl: last.DownloadUrl,
	}, nil
}

func (c *Collector) ListSnapshots(_ context.Context, _ *grpcapi.Empty) (*grpcapi.ListSnapshotsResponse, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	out := append([]*grpcapi.SnapshotInfo(nil), c.snapshots...)
	return &grpcapi.ListSnapshotsResponse{Snapshots: out}, nil
}

func (c *Collector) SetReady(ready bool) {
	c.ready.Store(ready)
}

func (c *Collector) Run(ctx context.Context, grpcAddr, httpAddr string) error {
	c.SetReady(true)
	defer c.SetReady(false)

	var grpcOpts []grpc.ServerOption
	if grpctls.ServerEnabled() {
		creds, err := grpctls.ServerCredentials()
		if err != nil {
			return err
		}
		grpcOpts = append(grpcOpts, grpc.Creds(creds))
		log.Printf("gRPC TLS enabled")
	}
	grpcSrv := grpc.NewServer(grpcOpts...)
	grpcapi.RegisterEventIngestServer(grpcSrv, c)
	grpcapi.RegisterCollectorAdminServer(grpcSrv, c)
	healthSrv := health.NewServer()
	healthSrv.SetServingStatus("", healthpb.HealthCheckResponse_SERVING)
	healthpb.RegisterHealthServer(grpcSrv, healthSrv)

	lis, err := net.Listen("tcp", grpcAddr)
	if err != nil {
		return err
	}
	go func() {
		log.Printf("dre-collector gRPC listening on %s", grpcAddr)
		if err := grpcSrv.Serve(lis); err != nil {
			log.Printf("gRPC server: %v", err)
		}
	}()

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	mux.HandleFunc("/readyz", func(w http.ResponseWriter, _ *http.Request) {
		if !c.ready.Load() {
			http.Error(w, "not leader", http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ready"))
	})
	mux.HandleFunc("/v1/events", c.serveIngestEvents)
	mux.HandleFunc("/v1/trigger", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var body struct{ Detail string }
		_ = json.NewDecoder(r.Body).Decode(&body)
		resp, _ := c.TriggerSnapshot(r.Context(), &grpcapi.TriggerSnapshotRequest{Reason: "manual", Detail: body.Detail})
		_ = json.NewEncoder(w).Encode(resp)
	})
	mux.HandleFunc("/v1/snapshots", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/snapshots" {
			c.serveSnapshotDownload(w, r)
			return
		}
		resp, _ := c.ListSnapshots(r.Context(), &grpcapi.Empty{})
		_ = json.NewEncoder(w).Encode(resp)
	})
	mux.HandleFunc("/v1/snapshots/", func(w http.ResponseWriter, r *http.Request) {
		c.serveSnapshotDownload(w, r)
	})
	httpSrv := &http.Server{Addr: httpAddr, Handler: mux}
	go func() {
		log.Printf("dre-collector HTTP admin on %s", httpAddr)
		if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("HTTP server: %v", err)
		}
	}()

	<-ctx.Done()
	healthSrv.SetServingStatus("", healthpb.HealthCheckResponse_NOT_SERVING)
	grpcSrv.GracefulStop()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = httpSrv.Shutdown(shutdownCtx)
	return nil
}

func (c *Collector) serveIngestEvents(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	var req struct {
		NodeID string `json:"node_id"`
		Events []struct {
			TimestampNs uint64 `json:"timestamp_ns"`
			Fd          uint32 `json:"fd"`
			IsWrite     uint8  `json:"is_write"`
			Comm        string `json:"comm"`
			Payload     string `json:"payload"`
		} `json:"events"`
	}
	if err := json.Unmarshal(body, &req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	nodeID := req.NodeID
	if nodeID == "" {
		nodeID = "local-dev"
	}
	for _, item := range req.Events {
		var evt ioevent.IOEvent
		evt.TimestampNs = item.TimestampNs
		if evt.TimestampNs == 0 {
			evt.TimestampNs = uint64(time.Now().UnixNano())
		}
		evt.Fd = item.Fd
		evt.IsWrite = item.IsWrite
		payload := []byte(item.Payload)
		if len(payload) > ioevent.MaxPayloadLen {
			payload = payload[:ioevent.MaxPayloadLen]
		}
		evt.PayloadLen = uint32(len(payload))
		copy(evt.Payload[:], payload)
		copy(evt.Comm[:], []byte(item.Comm))
		c.ingest.IngestNative(r.Context(), evt, nodeID)
	}
	w.WriteHeader(http.StatusAccepted)
	_, _ = w.Write([]byte(`{"status":"ok"}`))
}

func (c *Collector) serveSnapshotDownload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	rest := strings.TrimPrefix(r.URL.Path, "/v1/snapshots/")
	if !strings.HasSuffix(rest, "/download") {
		http.NotFound(w, r)
		return
	}
	id := strings.TrimSuffix(strings.TrimSuffix(rest, "/download"), "/")
	if id == "" {
		http.NotFound(w, r)
		return
	}

	c.mu.RLock()
	var info *grpcapi.SnapshotInfo
	for _, s := range c.snapshots {
		if s.Id == id {
			info = s
			break
		}
	}
	c.mu.RUnlock()
	if info == nil || info.Path == "" {
		http.NotFound(w, r)
		return
	}
	if info.DownloadUrl != "" && strings.HasPrefix(info.DownloadUrl, "http") && !strings.Contains(info.DownloadUrl, r.Host) {
		http.Redirect(w, r, info.DownloadUrl, http.StatusTemporaryRedirect)
		return
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	http.ServeFile(w, r, info.Path)
}

func envOr(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
