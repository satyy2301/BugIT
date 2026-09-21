package recordproxy

import (
	"testing"
)

func TestTruncateComm(t *testing.T) {
	long := "very-long-process-name-here"
	got := truncateComm(long)
	if len(got) > 16 {
		t.Fatalf("comm too long: %q", got)
	}
}

func TestParseHostPort(t *testing.T) {
	h, p := ParseHostPort("127.0.0.1:8080")
	if h != "127.0.0.1" || p != 8080 {
		t.Fatalf("got %s %d", h, p)
	}
}
