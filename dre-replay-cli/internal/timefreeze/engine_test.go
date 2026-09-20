package timefreeze_test

import (
	"testing"

	"github.com/bugit/dre-engine/api/ioevent"
	"github.com/bugit/dre-engine/api/manifest"
	"github.com/bugit/dre-engine/dre-replay-cli/internal/config"
	"github.com/bugit/dre-engine/dre-replay-cli/internal/timefreeze"
)

func TestSyncToCursorUsesTimeline(t *testing.T) {
	tl := manifest.ClockTimeline{
		Entries: []manifest.ClockEntry{
			{Index: 0, TimestampNs: 100},
			{Index: 5, TimestampNs: 500},
		},
	}
	eng := timefreeze.NewEngine(tl, config.TimeFreeze{Enabled: true})
	events := []ioevent.IOEvent{
		{TimestampNs: 100},
		{TimestampNs: 200},
	}
	eng.SyncToCursor(3, events)
	if eng.FrozenTimestampNs() != 100 {
		t.Fatalf("expected 100 at cursor 3, got %d", eng.FrozenTimestampNs())
	}
	eng.SyncToCursor(5, events)
	if eng.FrozenTimestampNs() != 500 {
		t.Fatalf("expected 500 at cursor 5, got %d", eng.FrozenTimestampNs())
	}
}
