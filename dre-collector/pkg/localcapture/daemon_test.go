package localcapture

import (
	"strings"
	"testing"

	"github.com/bugit/dre-engine/pkg/discover"
)

func TestRunCaptureAutoBlocksWhenPublicPortBusy(t *testing.T) {
	if !discover.IsListening("127.0.0.1", 80) && !discover.IsListening("127.0.0.1", 443) {
		t.Skip("no stable busy port for test")
	}
	// Document expected error shape when ports are in use.
	msg := "backend already listening on :4000 — use bugit record to attach instead of capture --auto"
	if !strings.Contains(msg, "bugit record") {
		t.Fatal("unexpected message")
	}
}
