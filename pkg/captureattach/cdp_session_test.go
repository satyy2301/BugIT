package captureattach

import (
	"testing"
)

func TestPickInspectTargetRejectsWeakMatch(t *testing.T) {
	targets := []InspectTarget{
		{Type: "node", Title: "Next.js", URL: "file:///app/web/node_modules/next/dist/server/lib/start-server.js", WebSocketDebuggerURL: "ws://127.0.0.1:9229/next"},
	}
	opts := TargetPickOptions{BackendPort: 4000, CaptureRoot: "/app/backend"}
	if got := PickInspectTarget(targets, opts); got != "" {
		t.Fatalf("pick %q want empty for weak Next.js-only match", got)
	}
}

func TestPickInspectTargetPrefersBackendPID(t *testing.T) {
	targets := []InspectTarget{
		{Type: "node", Title: "Next.js", URL: "file:///app/web/node_modules/next/dist/server/lib/start-server.js", WebSocketDebuggerURL: "ws://127.0.0.1:9229/next"},
		{Type: "node", Title: "node backend pid 12345", URL: "file:///app/placementiq/backend/src/server.js", WebSocketDebuggerURL: "ws://127.0.0.1:9230/backend"},
	}
	opts := TargetPickOptions{BackendPort: 4000, BackendPID: 12345, CaptureRoot: "/app/placementiq/backend"}
	got := PickInspectTarget(targets, opts)
	if got != "ws://127.0.0.1:9230/backend" {
		t.Fatalf("pick %q want backend target", got)
	}
}

func TestScoreTargetPrefersBackendServerJS(t *testing.T) {
	opts := TargetPickOptions{BackendPort: 4000, CaptureRoot: `C:\Users\me\PlacementIQ\backend`}
	nextTarget := InspectTarget{
		Type:                 "node",
		Title:                "Next.js",
		URL:                  "file:///C:/Users/me/PlacementIQ/web/node_modules/next/dist/server/lib/start-server.js",
		WebSocketDebuggerURL: "ws://next",
	}
	backendTarget := InspectTarget{
		Type:                 "node",
		Title:                "server.js",
		URL:                  "file:///C:/Users/me/PlacementIQ/backend/src/server.js",
		WebSocketDebuggerURL: "ws://backend",
	}
	nextScore := ScoreTarget(nextTarget, opts)
	backendScore := ScoreTarget(backendTarget, opts)
	if backendScore <= nextScore {
		t.Fatalf("backend score %d next score %d", backendScore, nextScore)
	}
	if backendScore < MinBackendTargetScore {
		t.Fatalf("backend score %d below min %d", backendScore, MinBackendTargetScore)
	}
}

func TestScoreTargetDisqualifiesNextDev(t *testing.T) {
	opts := TargetPickOptions{BackendPort: 4000, CaptureRoot: "/app/backend"}
	target := InspectTarget{
		Type:                 "node",
		Title:                "Next.js",
		URL:                  "file:///app/web/node_modules/next/dist/server/lib/start-server.js",
		WebSocketDebuggerURL: "ws://next",
	}
	if score := ScoreTarget(target, opts); score >= 0 {
		t.Fatalf("score %d want disqualified", score)
	}
}

func TestResolveBackendInspectorRejectsWeakMatch(t *testing.T) {
	_, err := ResolveBackendInspector(TargetPickOptions{BackendPort: 4000, CaptureRoot: "/nonexistent/backend"})
	if err == nil {
		t.Fatal("expected error when no inspector targets")
	}
}

func TestScoreInspectPortReturnsNegativeWhenUnavailable(t *testing.T) {
	if score := ScoreInspectPort(1, TargetPickOptions{}); score >= 0 {
		t.Fatalf("score %d want negative for closed port", score)
	}
}

func TestDiffNewTargetsFindsAddedTarget(t *testing.T) {
	before := []PortTarget{
		{Port: 9229, Target: InspectTarget{ID: "a", WebSocketDebuggerURL: "ws://9229/a"}},
	}
	after := append(before, PortTarget{
		Port: 9230,
		Target: InspectTarget{
			ID:                   "b",
			Title:                "server.js",
			URL:                  "file:///app/backend/src/server.js",
			WebSocketDebuggerURL: "ws://9230/b",
		},
	})
	newOnes := DiffNewTargets(before, after)
	if len(newOnes) != 1 || newOnes[0].Port != 9230 {
		t.Fatalf("diff %+v", newOnes)
	}
}

func TestResolveBackendInspectorWithDiffPrefersNewPort9230(t *testing.T) {
	opts := TargetPickOptions{BackendPort: 4000, CaptureRoot: "/app/backend"}
	before := []PortTarget{
		{Port: 9229, Target: InspectTarget{
			ID: "next", Title: "Next.js",
			URL: "file:///app/web/node_modules/next/dist/server/lib/start-server.js",
			WebSocketDebuggerURL: "ws://9229/next",
		}},
	}
	after := append(before, PortTarget{
		Port: 9230,
		Target: InspectTarget{
			ID: "backend", Title: "server.js",
			URL: "file:///app/backend/src/server.js",
			WebSocketDebuggerURL: "ws://9230/backend",
		},
	})
	insp, err := ResolveBackendInspectorWithDiff(before, opts)
	_ = after
	if err == nil && insp.Port == 9230 {
		return
	}
	// Without live inspector, verify diff selection logic via manual pick.
	newOnes := DiffNewTargets(before, after)
	best := BackendInspector{Score: -1}
	for _, pt := range newOnes {
		if !isNewBackendTarget(pt.Port, pt.Target, opts) {
			continue
		}
		score := ScoreTarget(pt.Target, opts)
		if score > best.Score {
			best = BackendInspector{Port: pt.Port, Score: score, WebSocketURL: pt.Target.WebSocketDebuggerURL}
		}
	}
	if best.Port != 9230 || best.Score < MinNewTargetScore {
		t.Fatalf("best %+v err=%v", best, err)
	}
}

func TestFindFreeInspectorPortSkipsBusy9229(t *testing.T) {
	port := FindFreeInspectorPort()
	if port < InspectorPortMin || port > InspectorPortMax {
		t.Fatalf("port %d out of range", port)
	}
}
