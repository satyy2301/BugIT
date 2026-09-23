package discover

import "strings"

// ProcessHasBugITPreload reports whether pid was started with BugIT preload hook.
func ProcessHasBugITPreload(pid int) bool {
	if pid <= 0 {
		return false
	}
	cmd := ProcessCommandLine(pid)
	return strings.Contains(cmd, ".bugit/preload") || strings.Contains(cmd, "preload.cjs")
}
