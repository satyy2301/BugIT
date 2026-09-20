package ingest_test

import (
	"testing"

	"github.com/bugit/dre-engine/api/ioevent"
	"github.com/bugit/dre-engine/dre-collector/internal/buffer"
	"github.com/bugit/dre-engine/dre-collector/internal/ingest"
	"github.com/bugit/dre-engine/dre-collector/internal/trigger"
	"github.com/bugit/dre-engine/dre-collector/internal/vector"
	"github.com/bugit/dre-engine/pkg/vectorclock"
)

func TestCrossPodVectorGraph(t *testing.T) {
	buf := buffer.New()
	vec := vector.New()
	svc := ingest.New(buf, trigger.New(nil), vec)

	var evt ioevent.IOEvent
	evt.IsWrite = 0
	header := "HTTP/1.1 200\r\nX-DRE-Vector-Clock: node-b:1\r\n\r\n"
	evt.PayloadLen = uint32(len(header))
	copy(evt.Payload[:], header)
	svc.IngestNative(nil, evt, "node-a")

	writeEvt := ioevent.IOEvent{IsWrite: 1}
	req := []byte("POST /x HTTP/1.1\r\nHost: svc\r\n\r\n")
	injected := vectorclock.MaybeInjectHTTP(req, "node-a", vectorclock.New())
	writeEvt.PayloadLen = uint32(len(injected))
	copy(writeEvt.Payload[:], injected)
	svc.IngestNative(nil, writeEvt, "node-a")

	graph := vec.Graph()
	if len(graph.Edges) == 0 {
		t.Fatal("expected cross-pod vector edge")
	}
}
