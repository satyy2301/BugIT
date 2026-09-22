package project

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFindCaptureRootMonorepo(t *testing.T) {
	root := t.TempDir()
	_ = os.MkdirAll(filepath.Join(root, "backend"), 0o755)
	_ = os.MkdirAll(filepath.Join(root, ".git"), 0o755)
	_ = os.WriteFile(filepath.Join(root, ".git", "HEAD"), []byte("ref"), 0o644)
	_ = os.WriteFile(filepath.Join(root, "backend", "package.json"), []byte(`{"scripts":{"dev":"nodemon"}}`), 0o644)

	got := FindCaptureRoot(root)
	want := filepath.Join(root, "backend")
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestDetectPublicPortFromEnv(t *testing.T) {
	root := t.TempDir()
	_ = os.WriteFile(filepath.Join(root, ".env"), []byte("PORT=4000\n"), 0o644)
	if p := DetectPublicPort(root); p != 4000 {
		t.Fatalf("got port %d", p)
	}
}

func TestDetectDevCommand(t *testing.T) {
	root := t.TempDir()
	_ = os.WriteFile(filepath.Join(root, "package.json"), []byte(`{"scripts":{"start":"node server.js"}}`), 0o644)
	if cmd := DetectDevCommand(root); cmd != "npm start" {
		t.Fatalf("got %q", cmd)
	}
}
