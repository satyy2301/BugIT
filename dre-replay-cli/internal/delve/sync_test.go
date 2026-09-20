package delve

import (
	"strings"
	"testing"

	"github.com/bugit/dre-engine/dre-replay-cli/internal/summary"
)

func TestFormatHint(t *testing.T) {
	h := FormatHint(Correlation{Index: 4, PID: 1234, Comm: "order-api"})
	if !strings.Contains(h, "Event 5") || !strings.Contains(h, "pid=1234") {
		t.Fatalf("unexpected hint: %s", h)
	}
}

func TestBuildCorrelations(t *testing.T) {
	events := []summary.EventSummary{{Index: 0, Pid: 99, Comm: "api", Service: "api"}}
	cor := BuildCorrelations(events)
	if cor[0].PID != 99 {
		t.Fatalf("expected pid 99, got %d", cor[0].PID)
	}
}
