package manifest

// SourceRef links a replay event index to a source location.
type SourceRef struct {
	Index    int    `json:"index"`
	File     string `json:"file"`
	Line     int    `json:"line"`
	Column   int    `json:"column,omitempty"`
	Function string `json:"function,omitempty"`
}

// SourceMap is an optional sidecar stored as source_map.json in .dre archives.
type SourceMap struct {
	Events []SourceRef `json:"events"`
}

func (m *SourceMap) RefForIndex(index int) *SourceRef {
	for i := range m.Events {
		if m.Events[i].Index == index {
			return &m.Events[i]
		}
	}
	return nil
}
