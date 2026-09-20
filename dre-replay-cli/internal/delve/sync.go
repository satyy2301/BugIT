package delve

import (
	"fmt"
	"log"

	"github.com/bugit/dre-engine/dre-replay-cli/internal/summary"
)

// Correlation links a replay event index to process metadata for Delve operators.
type Correlation struct {
	Index       int
	PID         uint32
	TID         uint32
	Comm        string
	Service     string
	TimestampNs uint64
}

// BuildCorrelations indexes summarized events for cursor→process hints.
func BuildCorrelations(events []summary.EventSummary) map[int]Correlation {
	out := make(map[int]Correlation, len(events))
	for _, e := range events {
		out[e.Index] = Correlation{
			Index:       e.Index,
			PID:         e.Pid,
			TID:         e.Tid,
			Comm:        e.Comm,
			Service:     e.Service,
			TimestampNs: e.TimestampNs,
		}
	}
	return out
}

// FormatHint returns a short status-bar friendly string.
func FormatHint(c Correlation) string {
	comm := c.Comm
	if comm == "" {
		comm = c.Service
	}
	return fmt.Sprintf("Event %d · pid=%d · comm=%s", c.Index+1, c.PID, comm)
}

// LogSeek emits correlation info when the replay cursor moves (MVP Delve bridge).
func LogSeek(index int, cor map[int]Correlation) {
	if c, ok := cor[index]; ok {
		log.Printf("delve-sync: %s tid=%d ts=%d", FormatHint(c), c.TID, c.TimestampNs)
	}
}
