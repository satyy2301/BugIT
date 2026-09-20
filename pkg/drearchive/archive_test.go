package drearchive_test

import (
	"bytes"
	"testing"

	"github.com/bugit/dre-engine/api/ioevent"
	"github.com/bugit/dre-engine/api/manifest"
	"github.com/bugit/dre-engine/pkg/drearchive"
)

func TestPackEncryptDecrypt(t *testing.T) {
	m := manifest.Manifest{ID: "test", Cluster: "local", EventCount: 1}
	var buf bytes.Buffer
	var evt ioevent.IOEvent
	evt.Fd = 3
	evt.PayloadLen = 4
	copy(evt.Payload[:], []byte("ping"))
	_ = evt.Encode(&buf)

	tar, err := drearchive.PackTarGz(m, buf.Bytes(), manifest.VectorGraph{}, manifest.RedactionLog{})
	if err != nil {
		t.Fatal(err)
	}
	enc, err := drearchive.Encrypt(tar, "test-key")
	if err != nil {
		t.Fatal(err)
	}
	plain, err := drearchive.Decrypt(enc, "test-key")
	if err != nil {
		t.Fatal(err)
	}
	snap, err := drearchive.ParseTarGz(plain)
	if err != nil {
		t.Fatal(err)
	}
	if len(snap.Events) != 1 || snap.Events[0].Fd != 3 {
		t.Fatalf("unexpected events: %+v", snap.Events)
	}
}

func TestIOEventEncodeDecode(t *testing.T) {
	var evt ioevent.IOEvent
	evt.PidTgid = 1
	evt.Fd = 3
	evt.PayloadLen = 4
	copy(evt.Payload[:], []byte("test"))

	var buf bytes.Buffer
	if err := evt.Encode(&buf); err != nil {
		t.Fatal(err)
	}
	out, err := ioevent.Decode(&buf)
	if err != nil {
		t.Fatal(err)
	}
	if out.Fd != evt.Fd {
		t.Fatalf("fd mismatch: %d vs %d", out.Fd, evt.Fd)
	}
}
