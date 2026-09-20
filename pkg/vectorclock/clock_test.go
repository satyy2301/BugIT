package vectorclock_test

import (
	"strings"
	"testing"

	"github.com/bugit/dre-engine/pkg/vectorclock"
)

func TestMaybeInjectHTTPAfterRequestLine(t *testing.T) {
	eng := vectorclock.New()
	payload := []byte("GET /api HTTP/1.1\r\nHost: example.com\r\n\r\n")
	out := vectorclock.MaybeInjectHTTP(payload, "node-a", eng)
	if !strings.HasPrefix(string(out), "GET /api HTTP/1.1\r\n") {
		t.Fatal("request line must stay first")
	}
	if !strings.Contains(string(out), vectorclock.HeaderName+": node-a:1") {
		t.Fatal("expected vector header after request line")
	}
	idx := strings.Index(string(out), vectorclock.HeaderName)
	hostIdx := strings.Index(string(out), "Host:")
	if idx == -1 || hostIdx == -1 || idx > hostIdx {
		t.Fatalf("vector header should precede Host, got %q", out)
	}
}
