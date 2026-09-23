package localcapture

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/bugit/dre-engine/api/ioevent"
	"github.com/bugit/dre-engine/pkg/captureattach"
	"github.com/bugit/dre-engine/pkg/debugbridge"
	"github.com/bugit/dre-engine/pkg/discover"
	"github.com/bugit/dre-engine/pkg/project"
	"github.com/bugit/dre-engine/pkg/runtimedetect"
)

// RunAttachRecord attaches to a running backend via Node CDP and records HTTP traffic.
func RunAttachRecord(ctx context.Context, workspace string, disc discover.Result, saveOnExit bool, detail string) (string, error) {
	daemon, cfg, err := StartDaemon(ctx, workspace, DaemonOptions{SkipProxy: true})
	if err != nil {
		return "", err
	}
	defer daemon.Stop()

	if err := project.SyncPreloadHook(disc.CaptureRoot, cfg.CollectorHTTP); err != nil {
		fmt.Fprintf(os.Stderr, "WARN: preload sync: %v\n", err)
	}

	pickOpts := captureattach.TargetPickOptions{
		BackendPort: disc.BackendPort,
		BackendPID:  disc.BackendPID,
		CaptureRoot: disc.CaptureRoot,
	}

	var inspector captureattach.BackendInspector
	var attachErr error
	if disc.BackendPID > 0 {
		fmt.Fprintf(os.Stderr, "Enabling Node inspector on backend pid %d\n", disc.BackendPID)
		inspector, attachErr = captureattach.PrepareBackendInspector(disc.CaptureRoot, disc.BackendPID, pickOpts)
	} else {
		inspector, attachErr = captureattach.ResolveBackendInspector(pickOpts)
	}
	preloadActive := discover.ProcessHasBugITPreload(disc.BackendPID)
	if attachErr != nil {
		if preloadActive {
			fmt.Fprintf(os.Stdout, "BugIT capturing inbound HTTP on :%d (preload mode)\n", disc.BackendPort)
		} else {
			return "", fmt.Errorf("resolve backend inspector: %w", attachErr)
		}
	}

	rt := runtimedetect.Detect(disc.CaptureRoot)
	comm := string(rt.Runtime)
	if comm == "unknown" {
		comm = "app"
	}

	var eventCount int64
	sink := func(evt ioevent.IOEvent) {
		if err := daemon.PostIOEvent(evt); err != nil {
			log.Printf("attach ingest: %v", err)
		} else {
			eventCount++
		}
	}

	var tap *captureattach.AttachTap
	if attachErr == nil {
		var err error
		tap, err = captureattach.StartAttachTap(ctx, captureattach.AttachOptions{
			InspectPort:     inspector.Port,
			InspectorWSURL:  inspector.WebSocketURL,
			InspectorTarget: inspector,
			BackendPort:     disc.BackendPort,
			BackendPID:      disc.BackendPID,
			CaptureRoot:     disc.CaptureRoot,
			Comm:            comm,
			Sink:            sink,
			DisablePreload:  preloadActive,
		})
		if err != nil {
			if preloadActive {
				fmt.Fprintf(os.Stderr, "WARN: CDP attach failed (%v) — continuing with preload ingest\n", err)
			} else {
				return "", err
			}
		} else {
			defer tap.Close()
			if rt.Runtime == runtimedetect.RuntimeNode {
				daemon.debugger = debugbridge.NewNodeBridgeWS(inspector.WebSocketURL)
				if err := daemon.debugger.Connect(ctx); err != nil {
					fmt.Fprintf(os.Stderr, "WARN: debugger bridge: %v\n", err)
				}
			}
			target := tap.Target()
			fmt.Fprintf(os.Stdout, "BugIT attached to backend on :%d (inspector :%d)\n", disc.BackendPort, inspector.Port)
			fmt.Fprintf(os.Stdout, "BugIT capturing inbound + outbound HTTP on :%d — target %s (%s)\n", disc.BackendPort, target.Title, target.URL)
		}
	} else if project.IsDevScriptPatched(disc.CaptureRoot) {
		fmt.Fprintf(os.Stdout, "BugIT configured backend inspector — restart backend (%s), then Record again\n", project.RecommendedRestartCommand(disc.CaptureRoot))
	}

	<-ctx.Done()

	inboundCount := int64(0)
	if tap != nil {
		inboundCount = tap.InboundCount()
	}
	if inboundCount == 0 && eventCount == 0 {
		if project.IsDevScriptPatched(disc.CaptureRoot) && !preloadActive {
			fmt.Fprintf(os.Stderr, "WARN: no HTTP events captured — restart backend (%s), use the app, then Stop & Save again\n", project.RecommendedRestartCommand(disc.CaptureRoot))
		} else {
			fmt.Fprintf(os.Stderr, "WARN: no HTTP events captured — likely attached to wrong Node process earlier; retry Record after backend restart\n")
		}
	}

	var snapPath string
	var snapErr error
	if saveOnExit {
		time.Sleep(300 * time.Millisecond)
		snapPath, snapErr = daemon.triggerSnapshot("interrupt")
	}
	if snapPath == "" && saveOnExit {
		snapPath, snapErr = daemon.triggerSnapshot(detail)
	}
	if saveOnExit && snapPath == "" && snapErr != nil {
		return "", fmt.Errorf("snapshot failed: %w", snapErr)
	}
	_ = cfg
	return snapPath, nil
}

// TriggerSnapshotHTTP asks a running collector daemon to save a snapshot.
func TriggerSnapshotHTTP(workspace, detail string) (string, error) {
	captureRoot := project.FindCaptureRoot(workspace)
	layout := project.LayoutFor(captureRoot)
	cfg, err := project.LoadConfig(layout.ConfigPath)
	if err != nil {
		cfg = project.DefaultConfig()
	}
	d := &Daemon{
		layout:     layout,
		cfg:        cfg,
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
	return d.triggerSnapshot(detail)
}
