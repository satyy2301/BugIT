package project

import (
	"strings"
	"testing"
)

func TestPreloadPayloadShape(t *testing.T) {
	body := renderPreloadHook("http://127.0.0.1:28080")
	for _, want := range []string{
		"timestamp_ns",
		"is_write",
		"node_id",
		"/v1/events",
		"http.server.request.start",
		"http.server.response.finish",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("missing %q in preload hook", want)
		}
	}
}
