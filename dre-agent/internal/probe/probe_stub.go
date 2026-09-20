//go:build !linux

package probe

import (
	"context"

	"github.com/bugit/dre-engine/api/ioevent"
)

type Loader struct{}

func NewLoader() *Loader { return &Loader{} }

func (l *Loader) SetMarkerHandler(_ MarkerHandler) {}

func (l *Loader) Load() error { return nil }

func (l *Loader) Close() error { return nil }

func (l *Loader) Run(ctx context.Context, out chan<- ioevent.IOEvent) error {
	return runMockAgent(ctx, out)
}

func (l *Loader) SetBypass(enabled bool) {}

func (l *Loader) Bypassed() bool { return false }
