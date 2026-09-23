package captureattach

import (
	"testing"
)

func TestPickInspectTargetPrefersBackendPID(t *testing.T) {
	targets := []InspectTarget{
		{Type: "node", Title: "Next.js", WebSocketDebuggerURL: "ws://127.0.0.1:9229/next"},
		{Type: "node", Title: "node backend pid 12345", WebSocketDebuggerURL: "ws://127.0.0.1:9229/backend"},
	}
	got := PickInspectTarget(targets, TargetPickOptions{BackendPort: 4000, BackendPID: 12345, CaptureRoot: "/app/backend"})
	if got != "ws://127.0.0.1:9229/backend" {
		t.Fatalf("pick %q want backend target", got)
	}
}

func TestPickInspectTargetPrefersBackendPort(t *testing.T) {
	targets := []InspectTarget{
		{Type: "node", Title: "frontend :3000", WebSocketDebuggerURL: "ws://127.0.0.1:9229/fe"},
		{Type: "node", Title: "api :4000", WebSocketDebuggerURL: "ws://127.0.0.1:9229/be"},
	}
	got := PickInspectTarget(targets, TargetPickOptions{BackendPort: 4000})
	if got != "ws://127.0.0.1:9229/be" {
		t.Fatalf("pick %q want :4000 target", got)
	}
}

func TestScoreInspectPortReturnsNegativeWhenUnavailable(t *testing.T) {
	if score := ScoreInspectPort(1, TargetPickOptions{}); score >= 0 {
		t.Fatalf("score %d want negative for closed port", score)
	}
}
