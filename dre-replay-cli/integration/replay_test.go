package integration_test

import (
	"encoding/json"
	"fmt"
	"net"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/bugit/dre-engine/dre-replay-cli/internal/archive"
	"github.com/bugit/dre-engine/dre-replay-cli/internal/config"
	"github.com/bugit/dre-engine/dre-replay-cli/internal/orchestrator"
)

func openFixture(t *testing.T) *archive.Snapshot {
	fixture := filepath.Join("..", "..", "test", "fixtures", "demo-checkout-500.dre")
	snap, err := archive.Open(fixture, "dev-insecure-key-change-me")
	if err != nil {
		t.Skip("demo fixture missing; run make demo-snapshot")
	}
	return snap
}

func proxyRequest(addr string) (string, error) {
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return "", err
	}
	defer conn.Close()
	_, err = conn.Write([]byte("POST /charge HTTP/1.1\r\nHost: payment-service\r\n\r\n"))
	if err != nil {
		return "", err
	}
	buf := make([]byte, 4096)
	n, err := conn.Read(buf)
	if err != nil {
		return "", err
	}
	return string(buf[:n]), nil
}

func TestReplayProxyReturns500(t *testing.T) {
	snap := openFixture(t)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	_ = ln.Close()

	cfg := config.ReplayConfig{ProxyAddr: addr, DebugAddr: "127.0.0.1:0"}
	session := orchestrator.New(snap, cfg, orchestrator.Options{})
	if err := session.Start(t.Context()); err != nil {
		t.Fatal(err)
	}
	defer session.Stop()

	time.Sleep(100 * time.Millisecond)
	resp, err := proxyRequest(addr)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(resp, "500") {
		t.Fatalf("expected 500 response, got %q", resp)
	}
}

func TestReplayCursorChangesProxyResponse(t *testing.T) {
	snap := openFixture(t)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	_ = ln.Close()

	cfg := config.ReplayConfig{ProxyAddr: addr, DebugAddr: "127.0.0.1:0"}
	session := orchestrator.New(snap, cfg, orchestrator.Options{})
	if err := session.Start(t.Context()); err != nil {
		t.Fatal(err)
	}
	defer session.Stop()

	// Cursor at end — charge/500 pair already passed.
	session.Seek(len(snap.Events))
	resp, err := proxyRequest(addr)
	if err == nil && strings.Contains(resp, "500") {
		t.Fatal("expected no 500 when cursor past events")
	}

	// Rewind to before payment-service read (index 3 in demo fixture).
	session.Seek(3)
	time.Sleep(50 * time.Millisecond)
	resp, err = proxyRequest(addr)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(resp, "500") {
		t.Fatalf("expected 500 after seek to index 3, got %q", resp)
	}
}

func TestReplayMultiPortService(t *testing.T) {
	snap := openFixture(t)
	svcLn, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	_, portStr, _ := net.SplitHostPort(svcLn.Addr().String())
	_ = svcLn.Close()
	port, err := strconv.Atoi(portStr)
	if err != nil {
		t.Fatal(err)
	}

	cfg := config.ReplayConfig{
		ProxyAddr: "",
		DebugAddr: "127.0.0.1:0",
		Services: []config.ServiceRule{{
			Name:       "payment-service",
			LocalPort:  port,
			RemoteHost: "payment-service:443",
			Protocol:   "http",
		}},
	}
	session := orchestrator.New(snap, cfg, orchestrator.Options{})
	if err := session.Start(t.Context()); err != nil {
		t.Fatal(err)
	}
	defer session.Stop()

	session.Seek(3)
	time.Sleep(100 * time.Millisecond)
	resp, err := proxyRequest(fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(resp, "500") {
		t.Fatalf("expected 500 on service port, got %q", resp)
	}
}

func TestDebuggerSeekOverTCP(t *testing.T) {
	snap := openFixture(t)
	cfg := config.ReplayConfig{
		ProxyAddr: "127.0.0.1:0",
		DebugAddr: "127.0.0.1:0",
	}
	session := orchestrator.New(snap, cfg, orchestrator.Options{})
	if err := session.Start(t.Context()); err != nil {
		t.Fatal(err)
	}
	defer session.Stop()

	conn, err := net.Dial("tcp", session.DebugAddr())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	enc := json.NewEncoder(conn)
	dec := json.NewDecoder(conn)
	if err := enc.Encode(map[string]interface{}{"method": "Seek", "index": 4}); err != nil {
		t.Fatal(err)
	}
	var resp map[string]interface{}
	if err := dec.Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp["index"] != float64(4) {
		t.Fatalf("seek response: %v", resp)
	}

	if err := enc.Encode(map[string]interface{}{"method": "GetState"}); err != nil {
		t.Fatal(err)
	}
	if err := dec.Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp["index"] != float64(4) {
		t.Fatalf("getstate response: %v", resp)
	}
}

func TestTimeFreezeOnStep(t *testing.T) {
	snap := openFixture(t)
	if len(snap.ClockTimeline.Entries) == 0 {
		t.Skip("fixture has no clock timeline; run make demo-snapshot")
	}

	cfg := config.ReplayConfig{
		ProxyAddr:  "127.0.0.1:0",
		DebugAddr:  "127.0.0.1:0",
		TimeFreeze: config.TimeFreeze{Enabled: true},
	}
	session := orchestrator.New(snap, cfg, orchestrator.Options{})
	if err := session.Start(t.Context()); err != nil {
		t.Fatal(err)
	}
	defer session.Stop()

	first := session.FrozenTimestampNs()
	session.Seek(snap.ClockTimeline.Entries[len(snap.ClockTimeline.Entries)-1].Index)
	second := session.FrozenTimestampNs()
	if second <= first {
		t.Fatalf("expected frozen time to advance: first=%d second=%d", first, second)
	}
}
