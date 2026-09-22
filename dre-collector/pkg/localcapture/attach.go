package localcapture

import (
	"context"
	"fmt"
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
		_ = captureattach.EnableNodeInspect(disc.BackendPID)
		time.Sleep(800 * time.Millisecond)
		if p := discover.FindInspectPort(); p > 0 {
			inspectPort = p
		}
	}

	rt := runtimedetect.Detect(disc.CaptureRoot)
	comm := string(rt.Runtime)
	if comm == "unknown" {
		comm = "app"
	}

	sink := func(evt ioevent.IOEvent) {
		_ = daemon.PostIOEvent(evt)
	}

	tap, err := captureattach.StartNetworkTap(ctx, inspectPort, comm, sink)
	if err != nil {
		return "", err
	}
	defer tap.Close()

	if rt.Runtime == runtimedetect.RuntimeNode {
		daemon.debugger = debugbridge.NewNodeBridge(inspectPort)
		_ = daemon.debugger.Connect(ctx)
	}

	fmt.Fprintf(os.Stdout, "BugIT attached to backend on :%d (inspector :%d)\n", disc.BackendPort, inspectPort)

	<-ctx.Done()

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
