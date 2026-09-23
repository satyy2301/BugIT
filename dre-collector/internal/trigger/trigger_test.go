package trigger

import (
	"testing"

	"github.com/bugit/dre-engine/api/ioevent"
	"github.com/bugit/dre-engine/api/manifest"
)

func TestEvaluate4xxDisabledByDefault(t *testing.T) {
	var fired manifest.Trigger
	e := New(func(t manifest.Trigger) { fired = t })
	var evt ioevent.IOEvent
	evt.IsWrite = 0
	copy(evt.Payload[:], []byte("HTTP/1.1 409 Conflict\r\n\r\n"))
	evt.PayloadLen = uint32(len("HTTP/1.1 409 Conflict\r\n\r\n"))
	e.Evaluate(evt, "local-dev")
	if fired.Type != "" {
		t.Fatalf("unexpected trigger %q", fired.Type)
	}
}

func TestEvaluate4xxWhenEnabled(t *testing.T) {
	var fired manifest.Trigger
	e := New(func(t manifest.Trigger) { fired = t })
	e.SetTrigger4xx(true)
	var evt ioevent.IOEvent
	evt.IsWrite = 0
	copy(evt.Payload[:], []byte("HTTP/1.1 400 Bad Request\r\n\r\n"))
	evt.PayloadLen = uint32(len("HTTP/1.1 400 Bad Request\r\n\r\n"))
	e.Evaluate(evt, "local-dev")
	if fired.Type != manifest.TriggerHTTP4xx {
		t.Fatalf("trigger %q want %q", fired.Type, manifest.TriggerHTTP4xx)
	}
}
