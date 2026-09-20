package proxy

import (
	"net"
	"strconv"
	"testing"

	"github.com/bugit/dre-engine/api/ioevent"
	"github.com/bugit/dre-engine/dre-replay-cli/internal/config"
)

func TestMultiServerCursorAware(t *testing.T) {
	var readEvt, writeEvt ioevent.IOEvent
	readEvt.IsWrite = 0
	readEvt.PayloadLen = uint32(len("GET /a HTTP/1.1\r\nHost: svc\r\n\r\n"))
	copy(readEvt.Payload[:], "GET /a HTTP/1.1\r\nHost: svc\r\n\r\n")
	writeEvt.IsWrite = 1
	writeEvt.PayloadLen = uint32(len("HTTP/1.1 200 OK\r\n\r\n"))
	copy(writeEvt.Payload[:], "HTTP/1.1 200 OK\r\n\r\n")

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	_ = ln.Close()

	cursor := 1
	m := NewMulti(addr, nil, []ioevent.IOEvent{readEvt, writeEvt}, func() int { return cursor })
	if err := m.Start(); err != nil {
		t.Fatal(err)
	}
	defer m.Stop()

	resp := m.matchResponse([]byte("GET /a HTTP/1.1\r\nHost: svc\r\n\r\n"), "")
	if resp != nil {
		t.Fatal("cursor past events should not match")
	}

	cursor = 0
	resp = m.matchResponse([]byte("GET /a HTTP/1.1\r\nHost: svc\r\n\r\n"), "")
	if resp == nil {
		t.Fatal("expected match at cursor 0")
	}
}

func TestMultiServerListensOnEphemeralPort(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	_, portStr, _ := net.SplitHostPort(ln.Addr().String())
	_ = ln.Close()
	port, _ := strconv.Atoi(portStr)

	m := NewMulti("", []config.ServiceRule{{
		Name: "svc", LocalPort: port, RemoteHost: "svc:8080", Protocol: "http",
	}}, nil, func() int { return 0 })
	if err := m.Start(); err != nil {
		t.Fatal(err)
	}
	defer m.Stop()
	if len(m.Addrs()) != 1 {
		t.Fatalf("expected one listener, got %v", m.Addrs())
	}
}

func TestMultiServerHostFilter(t *testing.T) {
	var readEvt, writeEvt ioevent.IOEvent
	readEvt.IsWrite = 0
	readEvt.PayloadLen = uint32(len("GET /pay HTTP/1.1\r\nHost: payment-service\r\n\r\n"))
	copy(readEvt.Payload[:], "GET /pay HTTP/1.1\r\nHost: payment-service\r\n\r\n")
	writeEvt.IsWrite = 1
	writeEvt.PayloadLen = uint32(len("HTTP/1.1 500\r\n\r\n"))
	copy(writeEvt.Payload[:], "HTTP/1.1 500\r\n\r\n")

	m := NewMulti("", []config.ServiceRule{{
		Name: "payment-service", LocalPort: 0, RemoteHost: "payment-service:443", Protocol: "http",
	}}, []ioevent.IOEvent{readEvt, writeEvt}, func() int { return 0 })

	resp := m.matchResponse([]byte("GET /pay HTTP/1.1\r\nHost: payment-service\r\n\r\n"), "payment-service:443")
	if resp == nil {
		t.Fatal("expected host-filtered match")
	}
	resp = m.matchResponse([]byte("GET /pay HTTP/1.1\r\nHost: other\r\n\r\n"), "payment-service:443")
	if resp != nil {
		t.Fatal("wrong host should not match")
	}
}
