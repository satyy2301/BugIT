package discover

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/bugit/dre-engine/pkg/project"
)

func TestDiscoverMonorepoPorts(t *testing.T) {
	root := t.TempDir()
	backend := filepath.Join(root, "backend")
	web := filepath.Join(root, "web")
	_ = os.MkdirAll(backend, 0o755)
	_ = os.MkdirAll(web, 0o755)
	_ = os.WriteFile(filepath.Join(backend, "package.json"), []byte(`{"scripts":{"dev":"nodemon"}}`), 0o644)
	_ = os.WriteFile(filepath.Join(backend, ".env"), []byte("PORT=4000\n"), 0o644)
	_ = os.WriteFile(filepath.Join(web, ".env"), []byte("NEXT_PUBLIC_API_URL=http://localhost:4000\nNEXTAUTH_URL=http://localhost:3000\n"), 0o644)

	got := Discover(root)
	if got.CaptureRoot != backend {
		t.Fatalf("capture root %q want %q", got.CaptureRoot, backend)
	}
	if got.BackendPort != 4000 {
		t.Fatalf("backend port %d want 4000", got.BackendPort)
	}
	if got.FrontendPort != 3000 {
		t.Fatalf("frontend port %d want 3000", got.FrontendPort)
	}
}

func TestIsListeningClosedPort(t *testing.T) {
	if IsListening("127.0.0.1", 19) {
		// chargen is unlikely; if somehow open skip
		t.Skip("port 19 unexpectedly open")
	}
}

func TestDetectPublicPortStillUsed(t *testing.T) {
	root := t.TempDir()
	_ = os.WriteFile(filepath.Join(root, ".env"), []byte("PORT=5001\n"), 0o644)
	if p := project.DetectPublicPort(root); p != 5001 {
		t.Fatalf("got %d", p)
	}
}
