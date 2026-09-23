package project

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSyncDevScriptPatchesNodemonOnce(t *testing.T) {
	root := t.TempDir()
	_ = os.WriteFile(filepath.Join(root, "package.json"), []byte(`{
  "scripts": {
    "dev": "nodemon src/server.js"
  }
}`), 0o644)

	port, patched, msg, err := SyncDevScript(root)
	if err != nil {
		t.Fatal(err)
	}
	if !patched {
		t.Fatal("expected patched dev script")
	}
	if port < 9229 || port > 9239 {
		t.Fatalf("inspect port %d", port)
	}
	if msg == "" || !strings.Contains(msg, "restart backend") {
		t.Fatalf("msg %q", msg)
	}

	data, _ := os.ReadFile(filepath.Join(root, "package.json"))
	var pkg struct {
		Scripts map[string]string `json:"scripts"`
	}
	_ = json.Unmarshal(data, &pkg)
	dev := pkg.Scripts["dev"]
	if !strings.Contains(dev, ".bugit/inspect-bootstrap.cjs") {
		t.Fatalf("dev script %q", dev)
	}
	if !strings.Contains(dev, ".bugit/preload.cjs") {
		t.Fatalf("dev script %q", dev)
	}
	if !strings.Contains(dev, "--inspect=127.0.0.1:") {
		t.Fatalf("dev script %q", dev)
	}
	if _, err := os.Stat(filepath.Join(root, DirName, preloadHookName)); err != nil {
		t.Fatalf("preload missing before restart: %v", err)
	}

	_, patched2, _, err := SyncDevScript(root)
	if err != nil {
		t.Fatal(err)
	}
	if patched2 {
		t.Fatal("expected idempotent sync")
	}
}

func TestSyncDevScriptPatchesNpmDev(t *testing.T) {
	root := t.TempDir()
	_ = os.WriteFile(filepath.Join(root, "package.json"), []byte(`{
  "scripts": {
    "dev": "npm run start:dev"
  }
}`), 0o644)

	port, patched, _, err := SyncDevScript(root)
	if err != nil {
		t.Fatal(err)
	}
	if !patched {
		t.Fatal("expected patched npm dev script")
	}

	data, _ := os.ReadFile(filepath.Join(root, "package.json"))
	var pkg struct {
		Scripts map[string]string `json:"scripts"`
	}
	_ = json.Unmarshal(data, &pkg)
	dev := pkg.Scripts["dev"]
	if !strings.Contains(dev, "node .bugit/run-with-hooks.cjs npm run start:dev") {
		t.Fatalf("dev script %q", dev)
	}
	if _, err := os.Stat(filepath.Join(root, DirName, runWithHooksName)); err != nil {
		t.Fatalf("run-with-hooks missing: %v", err)
	}

	cfg, err := LoadConfig(filepath.Join(root, DirName, ConfigName))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.InspectPort != port {
		t.Fatalf("inspect_port %d want %d", cfg.InspectPort, port)
	}
}

func TestSyncPreloadHookWritesFile(t *testing.T) {
	root := t.TempDir()
	if err := SyncPreloadHook(root, "http://127.0.0.1:28080"); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, DirName, preloadHookName)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "/v1/events") {
		t.Fatalf("preload missing ingest path: %s", data[:80])
	}
}

func TestPatchDevScriptNodemon(t *testing.T) {
	got, changed := patchDevScript("nodemon src/server.js", 9230)
	if !changed {
		t.Fatal("expected change")
	}
	if !strings.Contains(got, "-r .bugit/inspect-bootstrap.cjs") {
		t.Fatalf("got %q", got)
	}
	if !strings.Contains(got, "--inspect=127.0.0.1:9230") {
		t.Fatalf("got %q", got)
	}
}

func TestPatchDevScriptNpm(t *testing.T) {
	got, changed := patchDevScript("npm run dev", 9230)
	if !changed {
		t.Fatal("expected change")
	}
	if got != "node .bugit/run-with-hooks.cjs npm run dev" {
		t.Fatalf("got %q", got)
	}
}
