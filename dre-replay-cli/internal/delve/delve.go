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

func (b *Bridge) StartHeadless(binary string) error {
	b.cmd = exec.Command("dlv", "exec", binary, "--headless", "--listen=:2345", "--api-version=2", "--accept-multiclient")
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
