//go:build !windows

package discover

import (
	"os/exec"
	"strconv"
	"strings"
)

// PIDListeningOn returns the PID listening on host:port, or 0 if unknown.
func PIDListeningOn(host string, port int) int {
	out, err := exec.Command("lsof", "-nP", "-iTCP:"+itoa(port), "-sTCP:LISTEN", "-t").Output()
	if err != nil {
		return 0
	}
	line := strings.TrimSpace(string(out))
	if line == "" {
		return 0
	}
	first := strings.Split(line, "\n")[0]
	pid, err := strconv.Atoi(strings.TrimSpace(first))
	if err != nil {
		return 0
	}
	return pid
}
