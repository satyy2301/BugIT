package debugger

import (
	"encoding/json"
	"log"
	"net"
	"strings"
	"sync"

	"github.com/bugit/dre-engine/api/ioevent"
	"github.com/bugit/dre-engine/dre-replay-cli/internal/timefreeze"
)

// Controller is implemented by orchestrator.Session.
type Controller interface {
	Cursor() int
	Seek(index int) int
	Step(delta int) int
	Events() []ioevent.IOEvent
	SetBreakpoint(index int, enabled bool)
	ClearBreakpoints()
	StoppedReason(idx int) string
}

type Server struct {
	addr    string
	ctrl    Controller
	timeEng *timefreeze.Engine
	mu      sync.Mutex
	ln      net.Listener
}

func New(addr string, ctrl Controller) *Server {
	return &Server{addr: addr, ctrl: ctrl}
}

func (s *Server) SetTimeEngine(eng *timefreeze.Engine) {
	s.timeEng = eng
}

func (s *Server) Addr() string {
	if s.ln != nil {
		return s.ln.Addr().String()
	}
	return s.addr
}

func (s *Server) Start() error {
	ln, err := net.Listen("tcp", s.addr)
	if err != nil {
		return err
	}
	s.ln = ln
	log.Printf("debugger sync API on %s", ln.Addr())
	go s.serve(ln)
	return nil
}

func (s *Server) serve(ln net.Listener) {
	for {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		go s.handle(conn)
	}
}

func (s *Server) Stop() {
	if s.ln != nil {
		_ = s.ln.Close()
		s.ln = nil
	}
}

func (s *Server) handle(conn net.Conn) {
	defer conn.Close()
	dec := json.NewDecoder(conn)
	enc := json.NewEncoder(conn)
	for {
		var req struct {
			Method  string `json:"method"`
			Index   int    `json:"index"`
			Enabled bool   `json:"enabled"`
		}
		if err := dec.Decode(&req); err != nil {
			return
		}
		resp := s.dispatch(req.Method, req.Index, req.Enabled)
		if err := enc.Encode(resp); err != nil {
			return
		}
	}
}

func (s *Server) dispatch(method string, seekIndex int, enabled bool) map[string]interface{} {
	events := s.ctrl.Events()
	var idx int
	switch method {
	case "StepForward":
		idx = s.ctrl.Step(1)
	case "StepBackward":
		idx = s.ctrl.Step(-1)
	case "Seek":
		idx = s.ctrl.Seek(seekIndex)
	case "GetState":
		idx = s.ctrl.Cursor()
	case "RunToEvent":
		idx = s.runToIOEvent(s.ctrl.Cursor())
	case "SetBreakpoint":
		s.ctrl.SetBreakpoint(seekIndex, enabled)
		idx = s.ctrl.Cursor()
	case "ClearBreakpoints":
		s.ctrl.ClearBreakpoints()
		idx = s.ctrl.Cursor()
	default:
		idx = s.ctrl.Cursor()
	}

	var evt *ioevent.IOEvent
	if idx >= 0 && idx < len(events) {
		evt = &events[idx]
	}

	resp := map[string]interface{}{
		"index": idx,
		"total": len(events),
		"event": evt,
	}
	if evt != nil {
		resp["timestamp_ns"] = evt.TimestampNs
		resp["pid"] = uint32(evt.PidTgid >> 32)
		resp["tid"] = uint32(evt.PidTgid)
		comm := strings.TrimSpace(strings.TrimRight(string(evt.Comm[:]), "\x00"))
		if comm != "" {
			resp["comm"] = comm
		}
	}
	if reason := s.ctrl.StoppedReason(idx); reason != "" {
		resp["stopped_reason"] = reason
	}
	if s.timeEng != nil {
		resp["clock_index"] = s.timeEng.ClockIndex(idx)
		resp["frozen_timestamp_ns"] = s.timeEng.FrozenTimestampNs()
	}
	return resp
}

func (s *Server) runToIOEvent(start int) int {
	events := s.ctrl.Events()
	for i := start + 1; i < len(events); i++ {
		if s.ctrl.StoppedReason(i) == "breakpoint" {
			return s.ctrl.Seek(i)
		}
		evt := events[i]
		if evt.IsWrite <= 1 {
			return s.ctrl.Seek(i)
		}
	}
	return s.ctrl.Cursor()
}
