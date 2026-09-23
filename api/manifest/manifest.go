package manifest

import "time"

type TriggerType string

const (
	TriggerManual      TriggerType = "manual"
	TriggerHTTP5xx     TriggerType = "http_5xx"
	TriggerHTTP4xx     TriggerType = "http_4xx"
	TriggerProcessExit TriggerType = "process_exit"
	TriggerSIGSEGV     TriggerType = "sigsegv"
)

type Trigger struct {
	Type   TriggerType `json:"type"`
	Detail string      `json:"detail,omitempty"`
}

// Incident holds human-readable bug context for IDE display.
type Incident struct {
	Title      string   `json:"title"`
	Summary    string   `json:"summary"`
	RootCause  string   `json:"root_cause"`
	Services   []string `json:"services"`
	FailedStep string   `json:"failed_step"`
}

type Manifest struct {
	ID          string    `json:"id"`
	Cluster     string    `json:"cluster"`
	Nodes       []string  `json:"nodes"`
	Trigger     Trigger   `json:"trigger"`
	CapturedAt  time.Time `json:"captured_at"`
	EventCount  int       `json:"event_count"`
	Checksum    string    `json:"checksum"`
	Incident    *Incident `json:"incident,omitempty"`
}

type VectorGraph struct {
	Nodes []VectorNode `json:"nodes"`
	Edges []VectorEdge `json:"edges"`
}

type VectorNode struct {
	ID        string `json:"id"`
	NodeID    string `json:"node_id"`
	Sequence  uint64 `json:"sequence"`
	Timestamp uint64 `json:"timestamp_ns"`
}

type VectorEdge struct {
	From string `json:"from"`
	To   string `json:"to"`
}

type RedactionEntry struct {
	Offset int    `json:"offset"`
	Length int    `json:"length"`
	Rule   string `json:"rule"`
}

type RedactionLog struct {
	Entries []RedactionEntry `json:"entries"`
}

type ClockTimeline struct {
	Entries []ClockEntry `json:"entries"`
}

type ClockEntry struct {
	Index       int    `json:"index"`
	TimestampNs uint64 `json:"timestamp_ns"`
}
