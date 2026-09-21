package runtimedetect

import (
	"os"
	"path/filepath"
)

type Runtime string

const (
	RuntimeNode   Runtime = "node"
	RuntimeGo     Runtime = "go"
	RuntimePython Runtime = "python"
	RuntimeUnknown Runtime = "unknown"
)

type Info struct {
	Runtime Runtime
	Root    string
}

func Detect(root string) Info {
	if _, err := os.Stat(filepath.Join(root, "go.mod")); err == nil {
		return Info{Runtime: RuntimeGo, Root: root}
	}
	if _, err := os.Stat(filepath.Join(root, "package.json")); err == nil {
		return Info{Runtime: RuntimeNode, Root: root}
	}
	if hasFile(root, "pyproject.toml") || hasFile(root, "requirements.txt") {
		return Info{Runtime: RuntimePython, Root: root}
	}
	return Info{Runtime: RuntimeUnknown, Root: root}
}

func hasFile(root, name string) bool {
	_, err := os.Stat(filepath.Join(root, name))
	return err == nil
}

// EnvAdjustments returns env vars for zero-code debugger attach.
func (i Info) EnvAdjustments(inspectPort int) []string {
	switch i.Runtime {
	case RuntimeNode:
		return []string{"NODE_OPTIONS=--inspect=127.0.0.1:" + itoa(inspectPort)}
	case RuntimePython:
		return []string{"BUGIT_DEBUGPY=1", "BUGIT_DEBUGPY_PORT=" + itoa(inspectPort)}
	default:
		return nil
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [16]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}
