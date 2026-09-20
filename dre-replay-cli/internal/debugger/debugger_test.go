package debugger_test

import (
	"encoding/json"
	"net"
	"testing"

	"github.com/bugit/dre-engine/api/ioevent"
	"github.com/bugit/dre-engine/dre-replay-cli/internal/debugger"
)

type stubCtrl struct {
	idx         int
	events      []ioevent.IOEvent
	breakpoints map[int]bool
}

func (s *stubCtrl) Cursor() int               { return s.idx }
func (s *stubCtrl) Seek(i int) int            { s.idx = i; return s.idx }
func (s *stubCtrl) Step(d int) int            { s.idx += d; return s.idx }
func (s *stubCtrl) Events() []ioevent.IOEvent { return s.events }
func (s *stubCtrl) SetBreakpoint(index int, enabled bool) {
	if s.breakpoints == nil {
		s.breakpoints = make(map[int]bool)
	}
	if enabled {
		s.breakpoints[index] = true
	} else {
		delete(s.breakpoints, index)
	}
}
func (s *stubCtrl) ClearBreakpoints() { s.breakpoints = make(map[int]bool) }
func (s *stubCtrl) StoppedReason(idx int) string {
	if s.breakpoints[idx] {
		return "breakpoint"
	}
	return ""
}

func TestDebuggerSeekAndGetState(t *testing.T) {
	events := []ioevent.IOEvent{{IsWrite: 0}, {IsWrite: 1}}
	ctrl := &stubCtrl{events: events}
	srv := debugger.New("127.0.0.1:0", ctrl)
	if err := srv.Start(); err != nil {
		t.Fatal(err)
	}

	conn, err := net.Dial("tcp", srv.Addr())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	enc := json.NewEncoder(conn)
	dec := json.NewDecoder(conn)
	if err := enc.Encode(map[string]interface{}{"method": "Seek", "index": 1}); err != nil {
		t.Fatal(err)
	}
	var resp map[string]interface{}
	if err := dec.Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp["index"] != float64(1) {
		t.Fatalf("expected index 1, got %v", resp["index"])
	}
}

func TestDebuggerBreakpointOnStep(t *testing.T) {
	events := make([]ioevent.IOEvent, 6)
	ctrl := &stubCtrl{events: events}
	srv := debugger.New("127.0.0.1:0", ctrl)
	if err := srv.Start(); err != nil {
		t.Fatal(err)
	}

	conn, err := net.Dial("tcp", srv.Addr())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	enc := json.NewEncoder(conn)
	dec := json.NewDecoder(conn)

	if err := enc.Encode(map[string]interface{}{"method": "SetBreakpoint", "index": 2, "enabled": true}); err != nil {
		t.Fatal(err)
	}
	var resp map[string]interface{}
	if err := dec.Decode(&resp); err != nil {
		t.Fatal(err)
	}

	for i := 0; i < 2; i++ {
		if err := enc.Encode(map[string]interface{}{"method": "StepForward"}); err != nil {
			t.Fatal(err)
		}
		if err := dec.Decode(&resp); err != nil {
			t.Fatal(err)
		}
	}
	if resp["stopped_reason"] != "breakpoint" {
		t.Fatalf("expected breakpoint stop at index 2, got %v", resp)
	}
	if resp["index"] != float64(2) {
		t.Fatalf("expected index 2, got %v", resp["index"])
	}
}
