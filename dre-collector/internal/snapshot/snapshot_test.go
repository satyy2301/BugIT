package snapshot_test

import (
	"strings"
	"testing"

	"github.com/bugit/dre-engine/api/ioevent"
	"github.com/bugit/dre-engine/api/manifest"
	"github.com/bugit/dre-engine/dre-collector/internal/buffer"
	"github.com/bugit/dre-engine/dre-collector/internal/snapshot"
	"github.com/bugit/dre-engine/pkg/drearchive"
)

func TestClockTimelinePrefersClockMarkers(t *testing.T) {
	dir := t.TempDir()
	events := []buffer.Event{
		{IOEvent: ioevent.IOEvent{TimestampNs: 100, IsWrite: 0}},
		{IOEvent: ioevent.IOEvent{TimestampNs: 200, IsWrite: 3}},
		{IOEvent: ioevent.IOEvent{TimestampNs: 300, IsWrite: 1}},
	}
	exporter := snapshot.New(dir, "test", "ci-test-key")
	m, path, err := exporter.Export(events, manifest.VectorGraph{}, manifest.RedactionLog{}, manifest.Trigger{Type: manifest.TriggerManual})
	if err != nil {
		t.Fatal(err)
	}
	snap, err := drearchive.OpenFile(path, "ci-test-key")
	if err != nil {
		t.Fatal(err)
	}
	if len(snap.ClockTimeline.Entries) != 1 {
		t.Fatalf("expected one clock marker entry, got %d", len(snap.ClockTimeline.Entries))
	}
	if snap.ClockTimeline.Entries[0].TimestampNs != 200 {
		t.Fatalf("unexpected clock entry: %+v", snap.ClockTimeline.Entries[0])
	}
	if !strings.Contains(path, m.ID) {
		t.Fatal("path should include manifest id")
	}
}
