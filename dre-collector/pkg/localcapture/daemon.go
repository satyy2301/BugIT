package localcapture

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/bugit/dre-engine/api/ioevent"
	"github.com/bugit/dre-engine/api/manifest"
	"github.com/bugit/dre-engine/dre-collector/internal/leader"
	"github.com/bugit/dre-engine/dre-collector/internal/server"
	"github.com/bugit/dre-engine/pkg/debugbridge"
	"github.com/bugit/dre-engine/pkg/project"
	"github.com/bugit/dre-engine/pkg/recordproxy"
	"github.com/bugit/dre-engine/pkg/runtimedetect"
	"github.com/bugit/dre-engine/pkg/sourcemap"
)

type Daemon struct {
	layout     project.Layout
	cfg        project.Config
	collector  *server.Collector
	cancel     context.CancelFunc
	record     *recordproxy.Server
	outbound   *recordproxy.OutboundProxy
	sourceMap  *sourcemap.Store
	debugger   debugbridge.Bridge
	wg         sync.WaitGroup
	httpClient *http.Client
}

type CaptureOptions struct {
	Root       string
	Command    []string
	SaveOnExit bool
	Mode       string
	Detail     string
}

// DaemonOptions configures optional daemon behavior.
type DaemonOptions struct {
	SkipProxy bool
}

func StartDaemon(ctx context.Context, root string, opts DaemonOptions) (*Daemon, project.Config, error) {
	captureRoot := project.FindCaptureRoot(root)
	layout, err := project.EnsureLayout(captureRoot)
	if err != nil {
		return nil, project.Config{}, err
	}
	cfg, err := project.LoadConfig(layout.ConfigPath)
	if err != nil {
		return nil, project.Config{}, err
	}
	cfg = project.PrepareConfig(captureRoot, cfg)
	if err := project.SaveConfig(layout.ConfigPath, cfg); err != nil {
		return nil, project.Config{}, err
	}

	runCtx, cancel := context.WithCancel(ctx)
	d := &Daemon{
		layout:     layout,
		cfg:        cfg,
		sourceMap:  sourcemap.NewStore(),
		httpClient: &http.Client{Timeout: 30 * time.Second},
		cancel:     cancel,
	}

	os.Setenv("DRE_DATA_DIR", layout.DataPath)
	os.Setenv("DRE_CLUSTER", "local")
	os.Setenv("DRE_SNAPSHOT_KEY", cfg.SnapshotKey)
	os.Setenv("DRE_LEADER_ELECT", "0")
	os.Setenv("DRE_LOCAL_MODE", "1")

	grpcHost, grpcPort := splitHostPort(cfg.CollectorGRPC, "29090")
	httpHost, httpPort := splitHTTP(cfg.CollectorHTTP, "28080")
	grpcAddr := grpcHost + ":" + grpcPort
	httpAddr := httpHost + ":" + httpPort

	d.collector = server.New(layout.DataPath, "local", cfg.SnapshotKey, nil)
	d.collector.SetLocalHooks(server.LocalHooks{
		IncidentBuilder: BuildIncident,
		SourceMap:       d.sourceMap,
		ReplayWriter:    WriteReplayConfig,
		SnapshotDir:     layout.SnapshotsPath,
		LatestLink:      layout.LatestSnapshot,
		ReplayPath:      layout.ReplayPath,
		ProjectRoot:     captureRoot,
	})

	d.wg.Add(1)
	go func() {
		defer d.wg.Done()
		_ = leader.RunElection(runCtx, func(leadCtx context.Context) {
			if err := d.collector.Run(leadCtx, grpcAddr, httpAddr); err != nil {
				fmt.Fprintf(os.Stderr, "collector: %v\n", err)
			}
		})
	}()

	if err := waitReady(cfg.CollectorHTTP, 15*time.Second); err != nil {
		cancel()
		return nil, cfg, err
	}

	rt := runtimedetect.Detect(captureRoot)
	comm := string(rt.Runtime)
	if comm == "unknown" {
		comm = "app"
	}

	eventSink := func(evt ioevent.IOEvent) {
		_ = d.postEvent(evt)
		if isHTTPError(evt) {
			idx := int(evt.Fd)
			if ref := d.captureSourceRef(); ref != nil {
				d.sourceMap.Set(idx, ref.File, ref.Line, ref.Column, ref.Function)
			}
		}
	}

	if !opts.SkipProxy {
		internalPort := project.InternalPort(cfg.AppPort)
		targetAddr := fmt.Sprintf("127.0.0.1:%d", internalPort)
		d.record = recordproxy.New(cfg.RecordProxy, targetAddr, comm, "local-dev", eventSink)
		if err := d.record.Start(); err != nil {
			cancel()
			return nil, cfg, err
		}

		outboundAddr := "127.0.0.1:28082"
		d.outbound = recordproxy.NewOutbound(outboundAddr, comm, eventSink)
		if err := d.outbound.Start(); err != nil {
			cancel()
			return nil, cfg, err
		}

		if rt.Runtime == runtimedetect.RuntimeNode {
			d.debugger = debugbridge.NewNodeBridge(cfg.InspectPort)
			_ = d.debugger.Connect(runCtx)
		}
	}

	return d, cfg, nil
}

// PostIOEvent ingests an IO event through the collector with HTTP-error source mapping.
func (d *Daemon) PostIOEvent(evt ioevent.IOEvent) error {
	if err := d.postEvent(evt); err != nil {
		return err
	}
	if isHTTPError(evt) {
		idx := int(evt.Fd)
		if ref := d.captureSourceRef(); ref != nil {
			d.sourceMap.Set(idx, ref.File, ref.Line, ref.Column, ref.Function)
		}
	}
	return nil
}

func (d *Daemon) Stop() {
	d.cancel()
	if d.record != nil {
		_ = d.record.Stop()
	}
	if d.outbound != nil {
		_ = d.outbound.Stop()
	}
	if d.debugger != nil {
		d.debugger.Close()
	}
	d.wg.Wait()
}

func RunCapture(ctx context.Context, opts CaptureOptions) (string, error) {
	workspace, _ := filepath.Abs(opts.Root)
	if len(opts.Command) == 0 {
		return "", fmt.Errorf("capture command required after --")
	}

	captureRoot := project.FindCaptureRoot(workspace)
	daemon, cfg, err := StartDaemon(ctx, workspace, DaemonOptions{})
	if err != nil {
		return "", err
	}
	defer daemon.Stop()

	rt := runtimedetect.Detect(captureRoot)
	internalPort := project.InternalPort(cfg.AppPort)
	cmd := exec.CommandContext(ctx, opts.Command[0], opts.Command[1:]...)
	cmd.Dir = captureRoot
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	env := runtimedetect.CleanEnv(os.Environ())
	env = append(env, fmt.Sprintf("PORT=%d", internalPort))
	env = append(env, "BUGIT_RECORD_PROXY=http://127.0.0.1:28082")
	env = append(env, "HTTP_PROXY=http://127.0.0.1:28082")
	env = append(env, "HTTPS_PROXY=http://127.0.0.1:28082")
	env = append(env, rt.EnvAdjustments(cfg.InspectPort)...)
	cmd.Env = env

	fmt.Fprintf(os.Stdout, "BugIT recording at http://127.0.0.1:%d (use this URL as normal)\n", cfg.AppPort)

	if err := cmd.Start(); err != nil {
		return "", err
	}

	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()

	var snapPath string
	select {
	case err := <-done:
		if opts.SaveOnExit || err != nil {
			snapPath, _ = daemon.triggerSnapshot("capture exit")
		}
		if err != nil {
			return snapPath, fmt.Errorf("command exited: %w", err)
		}
	case <-ctx.Done():
		_ = cmd.Process.Signal(syscall.SIGINT)
		time.Sleep(500 * time.Millisecond)
		if opts.SaveOnExit {
			snapPath, _ = daemon.triggerSnapshot("interrupt")
		}
	}

	if snapPath == "" && opts.SaveOnExit {
		snapPath, _ = daemon.triggerSnapshot(opts.Detail)
	}
	return snapPath, nil
}

// RunCaptureAuto detects dev command and ports from workspace.
func RunCaptureAuto(ctx context.Context, workspace string, saveOnExit bool, detail string) (string, error) {
	target := project.ResolveCaptureTarget(workspace)
	return RunCapture(ctx, CaptureOptions{
		Root:       workspace,
		Command:    target.DevCommand,
		SaveOnExit: saveOnExit,
		Detail:     detail,
	})
}

// PublicURL returns the URL users should hit during capture.
func PublicURL(workspace string) string {
	target := project.ResolveCaptureTarget(workspace)
	return fmt.Sprintf("http://127.0.0.1:%d", target.PublicPort)
}

func (d *Daemon) TriggerSnapshotPublic(detail string) (string, error) {
	return d.triggerSnapshot(detail)
}

func (d *Daemon) triggerSnapshot(detail string) (string, error) {
	if detail == "" {
		detail = "manual capture"
	}
	body := strings.NewReader(fmt.Sprintf(`{"detail":%q}`, detail))
	resp, err := d.httpClient.Post(strings.TrimRight(d.cfg.CollectorHTTP, "/")+"/v1/trigger", "application/json", body)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	time.Sleep(300 * time.Millisecond)

	var parsed struct {
		Path       string `json:"path"`
		SnapshotID string `json:"snapshot_id"`
	}
	_ = json.Unmarshal(data, &parsed)
	if _, err := os.Stat(d.layout.LatestSnapshot); err == nil {
		return d.layout.LatestSnapshot, nil
	}
	if parsed.Path != "" {
		return parsed.Path, nil
	}
	return "", fmt.Errorf("snapshot trigger returned: %s", string(data))
}

func (d *Daemon) postEvent(evt ioevent.IOEvent) error {
	payload := map[string]interface{}{
		"node_id": "local-dev",
		"events": []map[string]interface{}{
			{
				"timestamp_ns": evt.TimestampNs,
				"fd":           evt.Fd,
				"is_write":     evt.IsWrite,
				"comm":         string(bytesTrim(evt.Comm[:])),
				"payload":      string(evt.Payload[:evt.PayloadLen]),
			},
		},
	}
	body, _ := json.Marshal(payload)
	resp, err := d.httpClient.Post(strings.TrimRight(d.cfg.CollectorHTTP, "/")+"/v1/events", "application/json", bytes.NewReader(body))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("ingest HTTP %d: %s", resp.StatusCode, b)
	}
	return nil
}

func (d *Daemon) captureSourceRef() *manifest.SourceRef {
	if d.debugger == nil {
		return nil
	}
	ref, err := d.debugger.TopFrame()
	if err != nil || ref == nil {
		return nil
	}
	return ref
}

func isHTTPError(evt ioevent.IOEvent) bool {
	if evt.IsWrite != 0 {
		return false
	}
	p := string(evt.Payload[:evt.PayloadLen])
	return strings.Contains(p, "HTTP/1.1 5") || strings.Contains(p, "HTTP/1.0 5") ||
		strings.Contains(p, "HTTP/1.1 4") || strings.Contains(p, "HTTP/1.0 4")
}

func waitReady(base string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	url := strings.TrimRight(base, "/") + "/healthz"
	for time.Now().Before(deadline) {
		resp, err := http.Get(url)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return nil
			}
		}
		time.Sleep(200 * time.Millisecond)
	}
	return fmt.Errorf("collector not ready at %s", url)
}

func splitHostPort(addr, defPort string) (host, port string) {
	if strings.Contains(addr, ":") {
		parts := strings.Split(addr, ":")
		return parts[0], parts[len(parts)-1]
	}
	return "127.0.0.1", defPort
}

func splitHTTP(raw, defPort string) (host, port string) {
	raw = strings.TrimPrefix(raw, "http://")
	raw = strings.TrimPrefix(raw, "https://")
	return splitHostPort(raw, defPort)
}

func bytesTrim(b []byte) []byte {
	return bytes.TrimRight(b, "\x00")
}

func LatestSnapshot(root string) (string, error) {
	layout := project.LayoutFor(project.FindCaptureRoot(root))
	if _, err := os.Stat(layout.LatestSnapshot); err == nil {
		return layout.LatestSnapshot, nil
	}
	entries, err := os.ReadDir(layout.SnapshotsPath)
	if err != nil {
		return "", err
	}
	var latest string
	var latestTime time.Time
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".dre") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		if info.ModTime().After(latestTime) {
			latestTime = info.ModTime()
			latest = filepath.Join(layout.SnapshotsPath, e.Name())
		}
	}
	if latest == "" {
		return "", fmt.Errorf("no snapshots in %s", layout.SnapshotsPath)
	}
	return latest, nil
}

func ListSnapshots(root string) ([]string, error) {
	layout := project.LayoutFor(project.FindCaptureRoot(root))
	entries, err := os.ReadDir(layout.SnapshotsPath)
	if err != nil {
		return nil, err
	}
	var out []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".dre") {
			out = append(out, filepath.Join(layout.SnapshotsPath, e.Name()))
		}
	}
	return out, nil
}
