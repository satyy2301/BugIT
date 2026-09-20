package proxy

import (
	"io"
	"log"
	"net"
	"sync"

	"github.com/bugit/dre-engine/api/ioevent"
)

type Server struct {
	addr   string
	events []ioevent.IOEvent
	mu     sync.Mutex
	cursor map[uint64]int
}

func New(addr string, events []ioevent.IOEvent) *Server {
	return &Server{addr: addr, events: events, cursor: make(map[uint64]int)}
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
	s.mu.Lock()
	idx := s.cursor[0]
	if idx >= len(s.events) {
		s.mu.Unlock()
		return
	}
	evt := s.events[idx]
	s.cursor[0] = idx + 1
	s.mu.Unlock()

	if evt.IsWrite == 1 {
		_, _ = conn.Write(evt.Payload[:evt.PayloadLen])
		return
	}
	buf := make([]byte, 4096)
	n, _ := conn.Read(buf)
	if n > 0 {
		_, _ = conn.Write(evt.Payload[:evt.PayloadLen])
	}
	_, _ = io.Copy(io.Discard, conn)
}
