//go:build !windows

package discover

import (
	"fmt"
	"os"
)

// ProcessCommandLine returns the command line for pid, or empty if unknown.
func ProcessCommandLine(pid int) string {
	if pid <= 0 {
		return ""
	}
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/cmdline", pid))
	if err != nil {
		return ""
	}
	return stringsReplaceNull(data)
}

func stringsReplaceNull(data []byte) string {
	for i, b := range data {
		if b == 0 {
			if i == 0 {
				return ""
			}
			return string(data[:i]) + " " + stringsReplaceNull(data[i+1:])
		}
	}
	return string(data)
}
