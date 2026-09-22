package captureattach

import (
	"fmt"
	"net/http"
	"os/exec"
	"time"
)

// EnableNodeInspect turns on the Node inspector for a running process.
func EnableNodeInspect(pid int) error {
	if pid <= 0 {
		return fmt.Errorf("invalid pid %d", pid)
	}
	cmd := exec.Command("node", "-e", fmt.Sprintf("process._debugProcess(%d)", pid))
	return cmd.Run()
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
