package snapshot

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/bugit/dre-engine/api/manifest"
	"github.com/bugit/dre-engine/dre-collector/internal/buffer"
	"github.com/bugit/dre-engine/pkg/drearchive"
	"github.com/google/uuid"
)

type Exporter struct {
	dir     string
	cluster string
	key     string
}

func New(dir, cluster, key string) *Exporter {
	return &Exporter{dir: dir, cluster: cluster, key: key}
}

func (e *Exporter) Export(events []buffer.Event, graph manifest.VectorGraph, redact manifest.RedactionLog, trig manifest.Trigger) (manifest.Manifest, string, error) {
	if err := os.MkdirAll(e.dir, 0o755); err != nil {
		return manifest.Manifest{}, "", err
	}

	id := uuid.New().String()
	manifestObj := manifest.Manifest{
		ID:         id,
		Cluster:    e.cluster,
		Nodes:      uniqueNodes(events),
		Trigger:    trig,
		CapturedAt: time.Now().UTC(),
		EventCount: len(events),
	}

	var eventsBuf bytes.Buffer
	for _, ev := range events {
		if err := ev.IOEvent.Encode(&eventsBuf); err != nil {
			return manifest.Manifest{}, "", err
		}
	}
	manifestObj.Checksum = sha256Hex(eventsBuf.Bytes())

	clock := buildClockTimeline(events)
	tarGz, err := drearchive.PackTarGz(manifestObj, eventsBuf.Bytes(), graph, redact, clock)
	if err != nil {
		return manifest.Manifest{}, "", err
	}

	encrypted, err := drearchive.Encrypt(tarGz, e.key)
	if err != nil {
		return manifest.Manifest{}, "", err
	}

	path := filepath.Join(e.dir, fmt.Sprintf("incident-%s.dre", id))
	if err := os.WriteFile(path, encrypted, 0o644); err != nil {
		return manifest.Manifest{}, "", err
	}
	return manifestObj, path, nil
}

func uniqueNodes(events []buffer.Event) []string {
	seen := map[string]bool{}
	var out []string
	for _, ev := range events {
		if ev.NodeID == "" || seen[ev.NodeID] {
			continue
		}
		seen[ev.NodeID] = true
		out = append(out, ev.NodeID)
	}
	return out
}

func buildClockTimeline(events []buffer.Event) manifest.ClockTimeline {
	var tl manifest.ClockTimeline
	for i, ev := range events {
		if ev.IOEvent.IsWrite == 2 {
			continue
		}
		tl.Entries = append(tl.Entries, manifest.ClockEntry{
			Index:       i,
			TimestampNs: ev.IOEvent.TimestampNs,
		})
	}
	return tl
}

func sha256Hex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}
