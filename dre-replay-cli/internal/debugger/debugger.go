package debugger

import (
	"encoding/json"
	"log"
	"net"
	"sync"

	"github.com/bugit/dre-engine/api/ioevent"
)

type Server struct {
	addr   string
	events []ioevent.IOEvent
	mu     sync.Mutex
	index  int
}

func New(addr string, events []ioevent.IOEvent) *Server {
	return &Server{addr: addr, events: events}
}

func (s *Server) Start() error {
	ln, err := net.Listen("unix", s.addr)
	if err != nil {
		// Fallback to TCP for cross-platform dev.
		ln, err = net.Listen("tcp", "127.0.0.1:19090")
		if err != nil {
			return err
		}
	}
	log.Printf("debugger sync API on %s", ln.Addr())
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
	dec := json.NewDecoder(conn)
	enc := json.NewEncoder(conn)
	for {
		var req struct {
			Method string `json:"method"`
		}
		if err := dec.Decode(&req); err != nil {
			return
		}
		resp := s.dispatch(req.Method)
		if err := enc.Encode(resp); err != nil {
			return
		}
	}
}

func (s *Server) dispatch(method string) map[string]interface{} {
	s.mu.Lock()
	defer s.mu.Unlock()
	switch method {
	case "StepForward":
		if s.index < len(s.events) {
			s.index++
		}
	case "StepBackward":
		if s.index > 0 {
			s.index--
		}
	}
	var evt *ioevent.IOEvent
	if s.index < len(s.events) {
		evt = &s.events[s.index]
	}
	return map[string]interface{}{
		"index": s.index,
		"total": len(s.events),
		"event": evt,
	}
}
