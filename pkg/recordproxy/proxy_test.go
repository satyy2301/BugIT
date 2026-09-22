package recordproxy

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/bugit/dre-engine/api/ioevent"
)

func TestReverseProxyPreservesResponseBody(t *testing.T) {
	const wantBody = `{"ok":true,"items":[1,2,3]}`
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(wantBody))
	}))
	defer upstream.Close()

	var events []ioevent.IOEvent
	var mu sync.Mutex
	srv := New("127.0.0.1:0", strings.TrimPrefix(upstream.URL, "http://"), "test", "local", func(evt ioevent.IOEvent) {
		mu.Lock()
		events = append(events, evt)
		mu.Unlock()
	})
	if err := srv.Start(); err != nil {
		t.Fatal(err)
	}
	defer srv.Stop()

	listenAddr := srv.BoundAddr()
	if listenAddr == "" {
		t.Fatal("proxy did not bind")
	}

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get("http://" + listenAddr + "/api/test")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	gotBody, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if string(gotBody) != wantBody {
		t.Fatalf("client body %q want %q", gotBody, wantBody)
	}

	time.Sleep(100 * time.Millisecond)
	mu.Lock()
	defer mu.Unlock()
	if len(events) < 2 {
		t.Fatalf("expected request+response events, got %d", len(events))
	}
	var responseRecorded bool
	for _, evt := range events {
		if evt.IsWrite != 0 {
			continue
		}
		payload := string(evt.Payload[:evt.PayloadLen])
		if strings.Contains(payload, wantBody) {
			responseRecorded = true
		}
	}
	if !responseRecorded {
		t.Fatal("response event missing body")
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [16]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}
