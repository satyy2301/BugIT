package vectorclock

import (
	"bytes"
	"fmt"
	"sync"
)

const HeaderName = "X-DRE-Vector-Clock"

type Engine struct {
	mu  sync.Mutex
	seq map[string]uint64
}

func New() *Engine {
	return &Engine{seq: make(map[string]uint64)}
}

func (e *Engine) InjectHeader(nodeID string) string {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.seq[nodeID]++
	return fmt.Sprintf("%s:%d", nodeID, e.seq[nodeID])
}

func MaybeInjectHTTP(payload []byte, nodeID string, eng *Engine) []byte {
	if len(payload) < 12 || eng == nil {
		return payload
	}
	if payload[0] != 'G' && payload[0] != 'P' && payload[0] != 'H' {
		return payload
	}
	header := fmt.Sprintf("%s: %s\r\n", HeaderName, eng.InjectHeader(nodeID))
	idx := bytes.Index(payload, []byte("\r\n"))
	if idx < 0 {
		return payload
	}
	out := make([]byte, 0, len(header)+len(payload))
	out = append(out, payload[:idx+2]...)
	out = append(out, header...)
	out = append(out, payload[idx+2:]...)
	return out
}
