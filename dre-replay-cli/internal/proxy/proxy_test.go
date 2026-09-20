package proxy

import (
	"testing"

	"github.com/bugit/dre-engine/api/ioevent"
)

func TestHTTPRequestKey(t *testing.T) {
	key := httpRequestKey([]byte("GET /checkout HTTP/1.1\r\nHost: x\r\n\r\n"))
	if key != "GET /checkout" {
		t.Fatalf("got %q", key)
	}
}

func TestMatchResponseByRequest(t *testing.T) {
	var readEvt, writeEvt ioevent.IOEvent
	readEvt.IsWrite = 0
	readEvt.PayloadLen = uint32(len("POST /charge HTTP/1.1\r\n\r\n"))
	copy(readEvt.Payload[:], "POST /charge HTTP/1.1\r\n\r\n")
	writeEvt.IsWrite = 1
	writeEvt.PayloadLen = uint32(len("HTTP/1.1 500 Internal Server Error\r\n\r\n"))
	copy(writeEvt.Payload[:], "HTTP/1.1 500 Internal Server Error\r\n\r\n")

	s := New("127.0.0.1:0", []ioevent.IOEvent{readEvt, writeEvt})
	resp := s.matchResponse([]byte("POST /charge HTTP/1.1\r\n\r\n"))
	if resp == nil {
		t.Fatal("expected matched response")
	}
	if string(resp) != "HTTP/1.1 500 Internal Server Error\r\n\r\n" {
		t.Fatalf("unexpected response: %q", string(resp))
	}
}
