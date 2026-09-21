package debugbridge

import (
	"context"

	"github.com/bugit/dre-engine/api/manifest"
)

// PythonBridge is a placeholder for debugpy stack capture.
type PythonBridge struct {
	port int
}

func NewPythonBridge(port int) *PythonBridge {
	return &PythonBridge{port: port}
}

func (p *PythonBridge) Connect(_ context.Context) error {
	return nil
}

func (p *PythonBridge) TopFrame() (*manifest.SourceRef, error) {
	return nil, nil
}

func (p *PythonBridge) Close() {}
