//go:build windows

package discover

import (
	"os/exec"
	"strconv"
	"strings"
)

// PIDListeningOn returns the PID listening on host:port, or 0 if unknown.
func PIDListeningOn(host string, port int) int {
	out, err := exec.Command("netstat", "-ano").Output()
	if err != nil {
		return 0
	}
	want := ":" + itoa(port)
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if !strings.Contains(line, want) || !strings.Contains(strings.ToUpper(line), "LISTENING") {
			continue
		}
		if !strings.Contains(line, host) && !strings.Contains(line, "0.0.0.0") && !strings.Contains(line, "[::]") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		pid, err := strconv.Atoi(fields[len(fields)-1])
		if err == nil && pid > 0 {
			return pid
		}
	}
	return 0
}
