package localcapture

import (
	"testing"

	"github.com/bugit/dre-engine/api/ioevent"
)

func TestBuildIncidentHTTP500(t *testing.T) {
	var e ioevent.IOEvent
	copy(e.Comm[:], []byte("node"))
	e.IsWrite = 0
	payload := "HTTP/1.1 500 Internal Server Error\r\nContent-Type: application/json\r\n\r\n{\"error\":\"db_down\"}"
	e.PayloadLen = uint32(len(payload))
	copy(e.Payload[:], payload)

	inc := BuildIncident([]ioevent.IOEvent{e})
	if inc == nil {
		t.Fatal("expected incident")
	}
	if inc.Title == "" {
		t.Fatal("expected title")
	}
	if inc.RootCause == "" {
		t.Fatal("expected root cause")
	}
}
