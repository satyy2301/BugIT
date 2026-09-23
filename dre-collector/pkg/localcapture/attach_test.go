package localcapture

import (
	"testing"

	"github.com/bugit/dre-engine/pkg/captureattach"
)

func TestAttachOptionsShape(t *testing.T) {
	opts := captureattach.AttachOptions{
		InspectPort: 9229,
		BackendPort: 4000,
		BackendPID:  123,
		CaptureRoot: "/repo/backend",
		Comm:        "node",
	}
	if opts.BackendPort != 4000 || opts.InspectPort != 9229 {
		t.Fatalf("unexpected attach options %+v", opts)
	}
}
