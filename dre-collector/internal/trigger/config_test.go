package trigger_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/bugit/dre-engine/dre-collector/internal/trigger"
)

func TestLoadRulesFromYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "rules.yaml")
	if err := os.WriteFile(path, []byte("http_5xx:\n  - \"HTTP/1.1 599\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	rules, err := trigger.LoadRules(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(rules) != 1 || rules[0] != "HTTP/1.1 599" {
		t.Fatalf("unexpected rules: %v", rules)
	}
}
