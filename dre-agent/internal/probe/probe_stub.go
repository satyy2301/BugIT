//go:build !linux

package probe

import (
	"context"
	"time"

	"github.com/bugit/dre-engine/api/ioevent"
)

type Loader struct{}

func NewLoader() *Loader { return &Loader{} }

func (l *Loader) Load() error { return nil }

func (l *Loader) Close() error { return nil }

func (l *Loader) Run(ctx context.Context, out chan<- ioevent.IOEvent) error {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	var seq uint64
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			seq++
			var e ioevent.IOEvent
			e.PidTgid = 1000
			e.TimestampNs = uint64(time.Now().UnixNano())
			e.Fd = 3
			e.PayloadLen = 12
			e.IsWrite = 1
			copy(e.Comm[:], []byte("mock-proc"))
			copy(e.Payload[:], []byte("GET /health"))
			out <- e
		}
	}
}

func (l *Loader) SetBypass(enabled bool) {}
