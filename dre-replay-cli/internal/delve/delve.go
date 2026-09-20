package delve

import (
	"fmt"
	"net"
	"os/exec"
	"time"
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
	host, portStr, err := net.SplitHostPort(listenAddr)
	if err != nil {
		host = "127.0.0.1"
		portStr = "2345"
	}
	if err := waitForTCP(host, portStr, 2*time.Second); err != nil {
		b.Stop()
		return fmt.Errorf("dlv not listening on %s: %w", listenAddr, err)
	}
	return nil
}

func waitForTCP(host, port string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", net.JoinHostPort(host, port), 200*time.Millisecond)
		if err == nil {
			conn.Close()
			return nil
		}
		time.Sleep(50 * time.Millisecond)
	}
	return fmt.Errorf("timeout waiting for delve on %s:%s", host, port)
}

func (b *Bridge) Stop() {
	if b.cmd != nil && b.cmd.Process != nil {
		_ = b.cmd.Process.Kill()
	}
}
