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

	inspectPort := disc.InspectPort
	if inspectPort <= 0 {
		inspectPort = cfg.InspectPort
	}
	if !captureattach.InspectAvailable(inspectPort) && disc.BackendPID > 0 {
		fmt.Fprintf(os.Stderr, "Enabling Node inspector on pid %d\n", disc.BackendPID)
		if err := captureattach.EnableNodeInspect(disc.BackendPID); err != nil {
			fmt.Fprintf(os.Stderr, "WARN: EnableNodeInspect: %v\n", err)
		}
		time.Sleep(800 * time.Millisecond)
		if p := discover.FindInspectPortForBackend(disc.BackendPort, disc.BackendPID, disc.CaptureRoot); p > 0 {
			inspectPort = p
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

	tap, err := captureattach.StartAttachTap(ctx, captureattach.AttachOptions{
		InspectPort: inspectPort,
		BackendPort: disc.BackendPort,
		BackendPID:  disc.BackendPID,
		CaptureRoot: disc.CaptureRoot,
		Comm:        comm,
		Sink:        sink,
	})
	if err != nil {
		return "", err
	}
	defer tap.Close()

	if rt.Runtime == runtimedetect.RuntimeNode {
		daemon.debugger = debugbridge.NewNodeBridge(inspectPort)
		if err := daemon.debugger.Connect(ctx); err != nil {
			fmt.Fprintf(os.Stderr, "WARN: debugger bridge: %v\n", err)
		}
	}

	fmt.Fprintf(os.Stdout, "BugIT attached to backend on :%d (inspector :%d)\n", disc.BackendPort, inspectPort)
	fmt.Fprintf(os.Stdout, "BugIT capturing inbound + outbound HTTP on :%d\n", disc.BackendPort)

	<-ctx.Done()

	if tap.InboundCount() == 0 && eventCount == 0 {
		fmt.Fprintf(os.Stderr, "WARN: no HTTP events captured — ensure Node >=18, API traffic hits :%d, and inspector is connected\n", disc.BackendPort)
	}

	var snapPath string
	if saveOnExit {
		time.Sleep(300 * time.Millisecond)
		snapPath, _ = daemon.triggerSnapshot("interrupt")
	}
	if snapPath == "" && saveOnExit {
		snapPath, _ = daemon.triggerSnapshot(detail)
	}
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
