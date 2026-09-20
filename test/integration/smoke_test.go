package integration_test

import (
	"testing"

	"github.com/bugit/dre-engine/api/ioevent"
)

func TestModuleSmoke(t *testing.T) {
	var e ioevent.IOEvent
	e.Fd = 1
	if e.Fd != 1 {
		t.Fatal("ioevent scaffold failed")
	}
}
