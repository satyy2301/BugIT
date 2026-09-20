package snapshot_test

import (
	"strings"
	"testing"

	"github.com/bugit/dre-engine/api/ioevent"
	"github.com/bugit/dre-engine/api/manifest"
	"github.com/bugit/dre-engine/dre-collector/internal/buffer"
	"github.com/bugit/dre-engine/dre-collector/internal/redact"
	"github.com/bugit/dre-engine/dre-collector/internal/snapshot"
	"github.com/bugit/dre-engine/pkg/drearchive"
)

func TestBearerNotInExportedSnapshot(t *testing.T) {
	dir := t.TempDir()
	var evt ioevent.IOEvent
	secret := "Bearer super-secret-token"
	evt.IsWrite = 1
	evt.PayloadLen = uint32(len("Authorization: " + secret))
	copy(evt.Payload[:], "Authorization: "+secret)

	scrubbed, _ := redact.Scrub(evt.Payload[:evt.PayloadLen])
	evt.PayloadLen = uint32(len(scrubbed))
	copy(evt.Payload[:], scrubbed)

	exporter := snapshot.New(dir, "test", "ci-test-key")
	_, path, err := exporter.Export(
		[]buffer.Event{{IOEvent: evt}},
		manifest.VectorGraph{},
		manifest.RedactionLog{},
		manifest.Trigger{Type: manifest.TriggerManual},
	)
	if err != nil {
		t.Fatal(err)
	}
	snap, err := drearchive.OpenFile(path, "ci-test-key")
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range snap.Events {
		payload := string(e.Payload[:e.PayloadLen])
		if strings.Contains(payload, "super-secret-token") {
			t.Fatal("bearer token leaked into exported snapshot")
		}
	}
}
