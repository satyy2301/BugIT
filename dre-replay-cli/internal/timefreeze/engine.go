package timefreeze

import (
	"fmt"
	"os"

	"github.com/bugit/dre-engine/api/ioevent"
	"github.com/bugit/dre-engine/api/manifest"
	"github.com/bugit/dre-engine/dre-replay-cli/internal/config"
)

// Engine maps debugger cursor position to frozen wall-clock time.
type Engine struct {
	timeline manifest.ClockTimeline
	cfg      config.TimeFreeze
	stateFile string
}

func NewEngine(timeline manifest.ClockTimeline, cfg config.TimeFreeze) *Engine {
	return &Engine{
		timeline:  timeline,
		cfg:       cfg,
		stateFile: os.Getenv("DRE_FROZEN_TIME_FILE"),
	}
}

func (e *Engine) SyncToCursor(cursor int, events []ioevent.IOEvent) {
	if !e.cfg.Enabled {
		return
	}
	ts := e.timestampForCursor(cursor, events)
	if ts == 0 {
		return
	}
	_ = os.Setenv("DRE_FROZEN_TIME_NS", fmt.Sprintf("%d", ts))
	if e.stateFile != "" {
		_ = os.WriteFile(e.stateFile, []byte(fmt.Sprintf("%d", ts)), 0o644)
	}
}

func (e *Engine) timestampForCursor(cursor int, events []ioevent.IOEvent) uint64 {
	if len(e.timeline.Entries) > 0 {
		var best uint64
		for _, ent := range e.timeline.Entries {
			if ent.Index <= cursor && ent.TimestampNs >= best {
				best = ent.TimestampNs
			}
		}
		if best > 0 {
			return best
		}
		return e.timeline.Entries[0].TimestampNs
	}
	if cursor >= 0 && cursor < len(events) {
		return events[cursor].TimestampNs
	}
	if len(events) > 0 {
		return events[0].TimestampNs
	}
	return 0
}

func (e *Engine) FrozenTimestampNs() uint64 {
	v := os.Getenv("DRE_FROZEN_TIME_NS")
	if v == "" {
		return 0
	}
	var ts uint64
	_, _ = fmt.Sscanf(v, "%d", &ts)
	return ts
}

func (e *Engine) ClockIndex(cursor int) int {
	if len(e.timeline.Entries) == 0 {
		return 0
	}
	idx := 0
	for i, ent := range e.timeline.Entries {
		if ent.Index <= cursor {
			idx = i
		}
	}
	return idx
}
