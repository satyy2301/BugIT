package delve

import (
	"fmt"
	"os/exec"
)

// Bridge launches Delve headless for replay debugging.
type Bridge struct {
	cmd *exec.Cmd
}

func New() *Bridge { return &Bridge{} }

func (b *Bridge) StartHeadless(binary, listenAddr string) error {
	if listenAddr == "" {
		listenAddr = "127.0.0.1:2345"
	}
	b.cmd = exec.Command("dlv", "exec", binary,
		"--headless", "--listen="+listenAddr,
		"--api-version=2", "--accept-multiclient")
	b.cmd.Stdout = nil
	b.cmd.Stderr = nil
	if err := b.cmd.Start(); err != nil {
		return fmt.Errorf("start dlv: %w (install: go install github.com/go-delve/delve/cmd/dlv@latest)", err)
	}
	return nil
}

func (b *Bridge) Stop() {
	if b.cmd != nil && b.cmd.Process != nil {
		_ = b.cmd.Process.Kill()
	}
}
