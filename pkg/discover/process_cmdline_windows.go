//go:build windows

package discover

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// ProcessCommandLine returns the command line for pid, or empty if unknown.
func ProcessCommandLine(pid int) string {
	if pid <= 0 {
		return ""
	}
	out, err := exec.Command("powershell", "-NoProfile", "-Command",
		fmt.Sprintf("(Get-CimInstance Win32_Process -Filter \"ProcessId=%d\").CommandLine", pid)).Output()
	if err == nil {
		line := strings.TrimSpace(string(out))
		if line != "" {
			return line
		}
	}
	out, err = exec.Command("wmic", "process", "where", "ProcessId="+strconv.Itoa(pid), "get", "CommandLine", "/value").Output()
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "CommandLine=") {
			return strings.TrimPrefix(line, "CommandLine=")
		}
	}
	return ""
}
