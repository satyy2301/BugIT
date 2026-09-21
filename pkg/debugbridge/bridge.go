package debugbridge

import (
	"context"

	"github.com/bugit/dre-engine/api/manifest"
)

// Bridge captures source locations from a runtime debugger without code changes.
type Bridge interface {
	Connect(ctx context.Context) error
	TopFrame() (*manifest.SourceRef, error)
	Close()
}
