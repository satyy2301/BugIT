package sourcemap

import (
	"encoding/json"
	"sync"

	"github.com/bugit/dre-engine/api/manifest"
)

type Store struct {
	mu   sync.Mutex
	refs []manifest.SourceRef
}

func NewStore() *Store {
	return &Store{}
}

func (s *Store) Set(index int, file string, line, column int, function string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.refs {
		if s.refs[i].Index == index {
			s.refs[i] = manifest.SourceRef{Index: index, File: file, Line: line, Column: column, Function: function}
			return
		}
	}
	s.refs = append(s.refs, manifest.SourceRef{
		Index: index, File: file, Line: line, Column: column, Function: function,
	})
}

func (s *Store) Snapshot() manifest.SourceMap {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]manifest.SourceRef, len(s.refs))
	copy(out, s.refs)
	return manifest.SourceMap{Events: out}
}

func (s *Store) RefForIndex(index int) *manifest.SourceRef {
	m := s.Snapshot()
	return m.RefForIndex(index)
}

func Marshal(m manifest.SourceMap) ([]byte, error) {
	return json.MarshalIndent(m, "", "  ")
}

func Unmarshal(data []byte) (manifest.SourceMap, error) {
	var m manifest.SourceMap
	if len(data) == 0 {
		return m, nil
	}
	err := json.Unmarshal(data, &m)
	return m, err
}
