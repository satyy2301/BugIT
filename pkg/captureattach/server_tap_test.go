package captureattach

import "testing"

func TestParseBindingMessageRequest(t *testing.T) {
	isWrite, wire, ok := ParseBindingMessage(`{"dir":1,"payload":"POST /auth/signin HTTP/1.1\r\nHost: localhost\r\n\r\n"}`)
	if !ok {
		t.Fatal("expected ok")
	}
	if isWrite != 1 {
		t.Fatalf("isWrite %d want 1", isWrite)
	}
	if !contains(string(wire), "POST /auth/signin") {
		t.Fatalf("wire %q", wire)
	}
}

func TestParseBindingMessageResponse(t *testing.T) {
	isWrite, wire, ok := ParseBindingMessage(`{"dir":0,"payload":"HTTP/1.1 409 Conflict\r\n\r\n"}`)
	if !ok {
		t.Fatal("expected ok")
	}
	if isWrite != 0 {
		t.Fatalf("isWrite %d want 0", isWrite)
	}
	if !contains(string(wire), "HTTP/1.1 409") {
		t.Fatalf("wire %q", wire)
	}
}

func TestParseBindingMessageInvalid(t *testing.T) {
	if _, _, ok := ParseBindingMessage("not-json"); ok {
		t.Fatal("expected invalid")
	}
}
