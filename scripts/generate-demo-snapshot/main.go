package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/bugit/dre-engine/pkg/demo"
	"github.com/bugit/dre-engine/pkg/drearchive"
	"github.com/google/uuid"
)

func main() {
	out := flag.String("out", "test/fixtures/demo-checkout-500.dre", "output path")
	key := flag.String("key", "dev-insecure-key-change-me", "encryption key")
	flag.Parse()

	events, graph, redact, m := demo.Checkout500()
	m.ID = uuid.New().String()
	m.EventCount = len(events)

	var eventsBuf bytes.Buffer
	for _, e := range events {
		if err := e.Encode(&eventsBuf); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}
	m.Checksum = sha256Hex(eventsBuf.Bytes())

	tarGz, err := drearchive.PackTarGz(m, eventsBuf.Bytes(), graph, redact, manifest.ClockTimeline{})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	enc, err := drearchive.Encrypt(tarGz, *key)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	if err := os.MkdirAll(filepath.Dir(*out), 0o755); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := os.WriteFile(*out, enc, 0o644); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Printf("wrote %s (%d events)\n", *out, len(events))
}

func sha256Hex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}
