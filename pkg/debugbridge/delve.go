package debugbridge

import (
	"context"

	"github.com/bugit/dre-engine/api/manifest"
)

// DelveBridge is a placeholder for Go Delve RPC stack capture.
type DelveBridge struct {
	addr string
}

func NewDelveBridge(addr string) *DelveBridge {
	return &DelveBridge{addr: addr}
}

func (d *DelveBridge) Connect(_ context.Context) error {
	return nil
}

func (d *DelveBridge) TopFrame() (*manifest.SourceRef, error) {
	return nil, nil
}

func (d *DelveBridge) Close() {}
