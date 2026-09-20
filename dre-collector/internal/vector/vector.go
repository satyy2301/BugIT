package vector

import (
	"fmt"
	"sync"

	"github.com/bugit/dre-engine/api/ioevent"
	"github.com/bugit/dre-engine/api/manifest"
)

const HeaderName = "X-DRE-Vector-Clock"

type Engine struct {
	mu    sync.Mutex
	seq   map[string]uint64
	nodes []manifest.VectorNode
	edges []manifest.VectorEdge
}

func New() *Engine {
	return &Engine{seq: make(map[string]uint64)}
}

func (e *Engine) Observe(nodeID string, evt ioevent.IOEvent) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.seq[nodeID]++
	seq := e.seq[nodeID]
	id := fmt.Sprintf("%s:%d", nodeID, seq)
	e.nodes = append(e.nodes, manifest.VectorNode{
		ID:        id,
		NodeID:    nodeID,
		Sequence:  seq,
		Timestamp: evt.TimestampNs,
	})
	if len(e.nodes) > 1 {
		prev := e.nodes[len(e.nodes)-2]
		e.edges = append(e.edges, manifest.VectorEdge{From: prev.ID, To: id})
	}
}

func (e *Engine) InjectHeader(nodeID string) string {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.seq[nodeID]++
	return fmt.Sprintf("%s:%d", nodeID, e.seq[nodeID])
}

func (e *Engine) ParseHeader(value string) (nodeID string, seq uint64, ok bool) {
	parts := splitClock(value)
	if len(parts) != 2 {
		return "", 0, false
	}
	var s uint64
	for _, c := range parts[1] {
		if c < '0' || c > '9' {
			return "", 0, false
		}
		s = s*10 + uint64(c-'0')
	}
	return parts[0], s, true
}

func (e *Engine) Graph() manifest.VectorGraph {
	e.mu.Lock()
	defer e.mu.Unlock()
	return manifest.VectorGraph{
		Nodes: append([]manifest.VectorNode(nil), e.nodes...),
		Edges: append([]manifest.VectorEdge(nil), e.edges...),
	}
}

func splitClock(v string) []string {
	for i := len(v) - 1; i >= 0; i-- {
		if v[i] == ':' {
			return []string{v[:i], v[i+1:]}
		}
	}
	return []string{v}
}

// MaybeInjectHTTP adds vector clock header to outbound HTTP payloads (userspace stub).
func MaybeInjectHTTP(payload []byte, nodeID string, eng *Engine) []byte {
	if len(payload) < 12 {
		return payload
	}
	if payload[0] != 'G' && payload[0] != 'P' && payload[0] != 'H' {
		return payload
	}
	header := fmt.Sprintf("%s: %s\r\n", HeaderName, eng.InjectHeader(nodeID))
	return append([]byte(header), payload...)
}
