package proxy

import (
	"bytes"
	"log"
	"net"
	"strings"
	"sync"

	"github.com/bugit/dre-engine/api/ioevent"
)

type Server struct {
	addr   string
	events []ioevent.IOEvent
	mu     sync.Mutex
	seq    int
}

func New(addr string, events []ioevent.IOEvent) *Server {
	return &Server{addr: addr, events: events}
}

func (s *Server) Start() error {
	ln, err := net.Listen("tcp", s.addr)
	if err != nil {
		return err
	}
	log.Printf("replay proxy listening on %s", s.addr)
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go s.handle(conn)
		}
	}()
	return nil
}

func (s *Server) handle(conn net.Conn) {
	defer conn.Close()
	buf := make([]byte, 8192)
	for {
		n, err := conn.Read(buf)
		if err != nil || n == 0 {
			return
		}
		request := append([]byte(nil), buf[:n]...)
		resp := s.matchResponse(request)
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

func isHTTPKeepAlive(request []byte) bool {
	return bytes.Contains(bytes.ToLower(request), []byte("connection: keep-alive"))
}

func (s *Server) matchResponse(request []byte) []byte {
	reqKey := requestKey(request)
	if reqKey == "" {
		return s.fallbackSequential()
	}
	reqHost := httpHost(request)

	s.mu.Lock()
	defer s.mu.Unlock()

	for i, evt := range s.events {
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
		for j := i + 1; j < len(s.events); j++ {
			if s.events[j].IsWrite == 1 && s.events[j].PayloadLen > 0 {
				out := make([]byte, s.events[j].PayloadLen)
				copy(out, s.events[j].Payload[:s.events[j].PayloadLen])
				return out
			}
		}
	}
	return nil
}

func requestKey(payload []byte) string {
	if k := grpcRequestKey(payload); k != "" {
		return k
	}
	return httpRequestKey(payload)
}

func (s *Server) fallbackSequential() []byte {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := s.seq; i < len(s.events); i++ {
		evt := s.events[i]
		if evt.IsWrite == 1 && evt.PayloadLen > 0 {
			s.seq = i + 1
			out := make([]byte, evt.PayloadLen)
			copy(out, evt.Payload[:evt.PayloadLen])
			return out
		}
	}
	return nil
}

func httpHost(payload []byte) string {
	lower := bytes.ToLower(payload)
	idx := bytes.Index(lower, []byte("host:"))
	if idx < 0 {
		return ""
	}
	rest := payload[idx+5:]
	end := bytes.Index(rest, []byte("\r\n"))
	if end < 0 {
		end = len(rest)
	}
	return strings.TrimSpace(string(rest[:end]))
}

func httpRequestKey(payload []byte) string {
	line := firstLine(payload)
	if line == "" {
		return ""
	}
	parts := strings.Fields(line)
	if len(parts) < 2 {
		return ""
	}
	method := strings.ToUpper(parts[0])
	path := parts[1]
	if idx := strings.Index(path, "?"); idx >= 0 {
		path = path[:idx]
	}
	return method + " " + path
}

func firstLine(payload []byte) string {
	payload = bytes.TrimSpace(payload)
	if len(payload) == 0 {
		return ""
	}
	if idx := bytes.IndexByte(payload, '\n'); idx >= 0 {
		return strings.TrimSpace(string(payload[:idx]))
	}
	return strings.TrimSpace(string(payload))
}
