package trigger



import (

	"bytes"

	"log"

	"sync"

	"time"



	"github.com/bugit/dre-engine/api/ioevent"

	"github.com/bugit/dre-engine/api/manifest"

)



type SnapshotFunc func(trigger manifest.Trigger)



type Engine struct {

	mu         sync.Mutex

	onSnap     SnapshotFunc

	rules5xx   []string

	rules4xx   []string

	trigger4xx bool

	cooldown   time.Duration

	lastFire   time.Time

}



func New(onSnap SnapshotFunc) *Engine {

	cd := 60 * time.Second

	return &Engine{

		onSnap:     onSnap,

		rules5xx:   []string{"HTTP/1.1 5", "HTTP/1.0 5"},

		rules4xx:   []string{"HTTP/1.1 4", "HTTP/1.0 4"},

		trigger4xx: false,

		cooldown:   cd,

	}

}



func (e *Engine) SetTrigger4xx(enabled bool) {

	e.mu.Lock()

	defer e.mu.Unlock()

	e.trigger4xx = enabled

}



func (e *Engine) Evaluate(evt ioevent.IOEvent, nodeID string) {

	if evt.IsWrite == 2 {

		return

	}

	if evt.IsWrite != 0 {

		return

	}

	payload := evt.Payload[:evt.PayloadLen]

	for _, rule := range e.rules5xx {

		if bytes.Contains(payload, []byte(rule)) {

			e.fire(manifest.TriggerHTTP5xx, "matched "+rule+" from "+nodeID)

			return

		}

	}

	e.mu.Lock()

	enabled4xx := e.trigger4xx

	e.mu.Unlock()

	if !enabled4xx {

		return

	}

	for _, rule := range e.rules4xx {

		if bytes.Contains(payload, []byte(rule)) {

			e.fire(manifest.TriggerHTTP4xx, "matched "+rule+" from "+nodeID)

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



func (e *Engine) SIGSEGV(detail string) {

	e.fire(manifest.TriggerSIGSEGV, detail)

}



func (e *Engine) fire(t manifest.TriggerType, detail string) {

	e.mu.Lock()

	defer e.mu.Unlock()

	if e.onSnap == nil {

		return

	}

	if (t == manifest.TriggerHTTP5xx || t == manifest.TriggerHTTP4xx) && time.Since(e.lastFire) < e.cooldown {

		return

	}

	e.lastFire = time.Now()

	log.Printf("snapshot trigger: %s %s", t, detail)

	e.onSnap(manifest.Trigger{Type: t, Detail: detail})

}

