package integration_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// TestReplayGate runs the canonical replay integration suite in dre-replay-cli/integration.
func TestReplayGate(t *testing.T) {
	root, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	repoRoot := filepath.Join(root, "..", "..")
	cmd := exec.Command("go", "test", "-v", "./dre-replay-cli/integration/...")
	cmd.Dir = repoRoot
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("replay integration failed: %v\n%s", err, out)
	}
}
