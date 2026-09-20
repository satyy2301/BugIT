package trigger

import (
	"bytes"
	"log"
	"sync"

	"github.com/bugit/dre-engine/api/ioevent"
	"github.com/bugit/dre-engine/api/manifest"
)

type SnapshotFunc func(trigger manifest.Trigger)

type Engine struct {
	mu      sync.Mutex
	onSnap  SnapshotFunc
	rules5xx []string
}

func New(onSnap SnapshotFunc) *Engine {
	return &Engine{
		onSnap:   onSnap,
		rules5xx: []string{"HTTP/1.1 5", "HTTP/1.0 5"},
	}
}

func (e *Engine) Evaluate(evt ioevent.IOEvent, nodeID string) {
	if evt.IsWrite == 2 {
		return
	}
	payload := evt.Payload[:evt.PayloadLen]
	for _, rule := range e.rules5xx {
		if bytes.Contains(payload, []byte(rule)) {
			e.fire(manifest.TriggerHTTP5xx, "matched "+rule+" from "+nodeID)
			return
		}
	}
}

func (e *Engine) Manual(detail string) {
	e.fire(manifest.TriggerManual, detail)
}

func (e *Engine) ProcessExit(detail string) {
	e.fire(manifest.TriggerProcessExit, detail)
}

func (e *Engine) fire(t manifest.TriggerType, detail string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.onSnap == nil {
		return
	}
	log.Printf("snapshot trigger: %s %s", t, detail)
	e.onSnap(manifest.Trigger{Type: t, Detail: detail})
}
