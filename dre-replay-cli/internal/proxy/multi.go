package proxy

import (
	"fmt"
	"log"
	"net"
	"strings"
	"sync"

	"github.com/bugit/dre-engine/api/ioevent"
	"github.com/bugit/dre-engine/dre-replay-cli/internal/config"
)

// MultiServer listens on the default proxy address and per-service local ports.
type MultiServer struct {
	defaultAddr string
	services    []config.ServiceRule
	events      []ioevent.IOEvent
	cursorFn    func() int

	mu       sync.Mutex
	cursor   int
	consumed map[int]bool
	listeners []net.Listener
}

func NewMulti(defaultAddr string, services []config.ServiceRule, events []ioevent.IOEvent, cursorFn func() int) *MultiServer {
	return &MultiServer{
		defaultAddr: defaultAddr,
		services:    services,
		events:      events,
		cursorFn:    cursorFn,
		consumed:    make(map[int]bool),
	}
}

func (m *MultiServer) SetCursor(index int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if index < m.cursor {
		for i := index; i < len(m.events); i++ {
			delete(m.consumed, i)
		}
	}
	m.cursor = index
}

func (m *MultiServer) Addrs() []string {
	var out []string
	for _, ln := range m.listeners {
		out = append(out, ln.Addr().String())
	}
	return out
}

func (m *MultiServer) Start() error {
	if m.defaultAddr != "" {
		if err := m.listen(m.defaultAddr, ""); err != nil {
			return err
		}
	}
	for _, svc := range m.services {
		addr := fmt.Sprintf("127.0.0.1:%d", svc.LocalPort)
		if err := m.listen(addr, svc.RemoteHost); err != nil {
			return err
		}
	}
	if len(m.listeners) == 0 {
		return fmt.Errorf("no proxy listeners configured")
	}
	return nil
}

func (m *MultiServer) listen(addr, hostFilter string) error {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	m.listeners = append(m.listeners, ln)
	log.Printf("replay proxy listening on %s (host=%s)", addr, hostFilterOrAny(hostFilter))
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go m.handle(conn, hostFilter)
		}
	}()
	return nil
}

func hostFilterOrAny(h string) string {
	if h == "" {
		return "*"
	}
	return h
}

func (m *MultiServer) Stop() {
	for _, ln := range m.listeners {
		_ = ln.Close()
	}
}

func (m *MultiServer) handle(conn net.Conn, hostFilter string) {
	defer conn.Close()
	buf := make([]byte, 8192)
	for {
		n, err := conn.Read(buf)
		if err != nil || n == 0 {
			return
		}
		request := append([]byte(nil), buf[:n]...)
		resp := m.matchResponse(request, hostFilter)
		if resp == nil {
			log.Printf("replay proxy: no match for %q", firstLine(request))
			return
		}
		_, _ = conn.Write(resp)
		if !isHTTPKeepAlive(request) {
			return
		}
	}
}

func (m *MultiServer) matchResponse(request []byte, hostFilter string) []byte {
	if m.cursorFn != nil {
		m.SetCursor(m.cursorFn())
	}

	reqKey := requestKey(request)
	reqHost := httpHost(request)
	if hostFilter != "" {
		filterHost := strings.Split(hostFilter, ":")[0]
		if reqHost != "" && !strings.EqualFold(reqHost, filterHost) && !strings.HasPrefix(reqHost, filterHost) {
			return nil
		}
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	minIdx := m.cursor

	if reqKey == "" {
		return m.fallbackFrom(minIdx)
	}

	for i := minIdx; i < len(m.events); i++ {
		if m.consumed[i] {
			continue
		}
		evt := m.events[i]
		if evt.IsWrite != 0 {
			continue
		}
		payload := evt.Payload[:evt.PayloadLen]
		if requestKey(payload) != reqKey {
			continue
		}
		if reqHost != "" && httpHost(payload) != "" && httpHost(payload) != reqHost {
			continue
		}
		for j := i + 1; j < len(m.events); j++ {
			w := m.events[j]
			if w.IsWrite == 1 && w.PayloadLen > 0 && !m.consumed[j] {
				m.consumed[j] = true
				out := make([]byte, w.PayloadLen)
				copy(out, w.Payload[:w.PayloadLen])
				return out
			}
		}
	}
	return nil
}

func (m *MultiServer) fallbackFrom(minIdx int) []byte {
	for i := minIdx; i < len(m.events); i++ {
		if m.consumed[i] {
			continue
		}
		evt := m.events[i]
		if evt.IsWrite == 1 && evt.PayloadLen > 0 {
			m.consumed[i] = true
			out := make([]byte, evt.PayloadLen)
			copy(out, evt.Payload[:evt.PayloadLen])
			return out
		}
	}
	return nil
}
