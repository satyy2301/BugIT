package captureattach

import (
	"testing"

	"github.com/bugit/dre-engine/api/ioevent"
)

func TestHandleResponseFormatsHTTPError(t *testing.T) {
	tap := &NetworkTap{comm: "node", sink: func(evt ioevent.IOEvent) {
		payload := string(evt.Payload[:evt.PayloadLen])
		if evt.IsWrite != 0 {
			t.Fatalf("response should be read direction")
		}
		if !contains(payload, "HTTP/1.1 404") {
			t.Fatalf("payload %q", payload)
		}
	}}
	tap.handleResponse(map[string]interface{}{
		"response": map[string]interface{}{
			"status":     float64(404),
			"statusText": "Not Found",
			"url":        "http://127.0.0.1:4000/auth/signin",
		},
	})
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 || indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
