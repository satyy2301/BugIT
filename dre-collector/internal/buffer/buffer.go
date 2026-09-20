package buffer

import (
	"sync"
	"time"

	"github.com/bugit/dre-engine/api/ioevent"
)

const WindowDuration = 30 * time.Second

type Event struct {
	IOEvent ioevent.IOEvent
	NodeID  string
	At      time.Time
}

type RollingBuffer struct {
	mu     sync.RWMutex
	events []Event
}

func New() *RollingBuffer {
	return &RollingBuffer{}
}

func (b *RollingBuffer) Add(evt ioevent.IOEvent, nodeID string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	now := time.Now()
	b.events = append(b.events, Event{IOEvent: evt, NodeID: nodeID, At: now})
	cutoff := now.Add(-WindowDuration)
	i := 0
	for ; i < len(b.events); i++ {
		if b.events[i].At.After(cutoff) {
			break
		}
	}
	if i > 0 {
		b.events = b.events[i:]
	}
}

func (b *RollingBuffer) Snapshot() []Event {
	b.mu.RLock()
	defer b.mu.RUnlock()
	out := make([]Event, len(b.events))
	copy(out, b.events)
	return out
}

func (b *RollingBuffer) Len() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return len(b.events)
}
