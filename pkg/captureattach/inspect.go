package captureattach

import (
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"time"

	"github.com/bugit/dre-engine/pkg/project"
)

// EnableNodeInspect turns on the Node inspector for a running process on the default port (9229).
func EnableNodeInspect(pid int) error {
	if pid <= 0 {
		return fmt.Errorf("invalid pid %d", pid)
	}
	cmd := exec.Command("node", "-e", fmt.Sprintf("process._debugProcess(%d)", pid))
	return cmd.Run()
}

// EnableNodeInspectOnFreePort enables the backend inspector, preferring a free port when :9229 is busy.
func EnableNodeInspectOnFreePort(pid int) (port int, err error) {
	if pid <= 0 {
		return 0, fmt.Errorf("invalid pid %d", pid)
	}
	port = FindFreeInspectorPort()
	if port == InspectorPortMin {
		return port, EnableNodeInspect(pid)
	}
	return port, fmt.Errorf("inspector port %d in use — backend must restart with BugIT inspect bootstrap on :%d", InspectorPortMin, port)
}

// InspectAvailable reports whether a Node inspector is listening.
func InspectAvailable(port int) bool {
	if port <= 0 {
		return false
	}
	client := &http.Client{Timeout: 400 * time.Millisecond}
	resp, err := client.Get(fmt.Sprintf("http://127.0.0.1:%d/json/list", port))
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

// PrepareBackendInspector syncs capture hooks, enables the inspector when possible, and resolves the debug target.
func PrepareBackendInspector(captureRoot string, backendPID int, opts TargetPickOptions) (BackendInspector, error) {
	if captureRoot == "" {
		captureRoot = opts.CaptureRoot
	}

	inspectPort, patched, syncMsg, err := project.SyncDevScript(captureRoot)
	if err != nil {
		return BackendInspector{}, fmt.Errorf("sync dev script: %w", err)
	}
	if syncMsg != "" {
		fmt.Fprintln(os.Stdout, syncMsg)
	}
	_ = patched

	if insp, err := ResolveBackendInspector(opts); err == nil {
		return insp, nil
	}

	before := SnapshotPortTargets()

	if backendPID > 0 {
		runtimePort, enableErr := EnableNodeInspectOnFreePort(backendPID)
		if enableErr != nil {
			if inspectPort > InspectorPortMin {
				fmt.Fprintf(os.Stderr, "WARN: Node inspector :%d blocked (likely Next.js) — BugIT configured backend on :%d\n", InspectorPortMin, inspectPort)
				fmt.Fprintf(os.Stdout, "BugIT configured backend inspector on :%d — restart backend once (Ctrl+C then npm run dev, or nodemon rs)\n", inspectPort)
			} else {
				fmt.Fprintf(os.Stderr, "WARN: EnableNodeInspect pid %d: %v\n", backendPID, enableErr)
			}
		} else if runtimePort > InspectorPortMin {
			_ = runtimePort
		}
	}

	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		if insp, err := ResolveBackendInspectorWithDiff(before, opts); err == nil {
			return insp, nil
		}
		if insp, err := ResolveBackendInspector(opts); err == nil {
			return insp, nil
		}
		time.Sleep(300 * time.Millisecond)
	}

	freePort := inspectPort
	if freePort <= 0 {
		freePort = FindFreeInspectorPort()
	}
	if freePort > InspectorPortMin {
		return BackendInspector{}, fmt.Errorf("no backend inspector target found — restart backend so BugIT can bind inspector on :%d, then Record again", freePort)
	}
	return BackendInspector{}, fmt.Errorf("no backend inspector target found (need score %d) — ensure backend is Node >=18", MinBackendTargetScore)
}
