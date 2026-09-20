package redact_test

import (
	"strings"
	"testing"

	"github.com/bugit/dre-engine/dre-collector/internal/redact"
)

func TestScrubBearerToken(t *testing.T) {
	payload := []byte("Authorization: Bearer secret-token-123\r\n")
	out, log := redact.Scrub(payload)
	if strings.Contains(string(out), "secret-token-123") {
		t.Fatalf("token leaked: %q", out)
	}
	if len(log.Entries) == 0 {
		t.Fatal("expected redaction log entry")
	}
}
