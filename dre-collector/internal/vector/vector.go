package vector

import (
	"fmt"
	"sync"

	"github.com/bugit/dre-engine/api/ioevent"
	"github.com/bugit/dre-engine/api/manifest"
	"github.com/bugit/dre-engine/pkg/vectorclock"
)

const HeaderName = vectorclock.HeaderName

type Engine struct {
	mu    sync.Mutex
	clock *vectorclock.Engine
	seq   map[string]uint64
	nodes []manifest.VectorNode
	edges []manifest.VectorEdge
}

func New() *Engine {
	return &Engine{
		clock: vectorclock.New(),
		seq:   make(map[string]uint64),
	}
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
	return e.clock.InjectHeader(nodeID)
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

func (e *Engine) MergeRemote(remoteNode string, seq uint64, localNode string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	remoteID := fmt.Sprintf("%s:%d", remoteNode, seq)
	localSeq := e.seq[localNode]
	localID := fmt.Sprintf("%s:%d", localNode, localSeq)
	e.nodes = append(e.nodes, manifest.VectorNode{
		ID:       remoteID,
		NodeID:   remoteNode,
		Sequence: seq,
	})
	e.edges = append(e.edges, manifest.VectorEdge{From: remoteID, To: localID})
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

func MaybeInjectHTTP(payload []byte, nodeID string, eng *Engine) []byte {
	if eng == nil {
		return payload
	}
	return vectorclock.MaybeInjectHTTP(payload, nodeID, eng.clock)
}
