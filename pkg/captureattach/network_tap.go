package captureattach

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/bugit/dre-engine/api/ioevent"
	"golang.org/x/net/websocket"
)

// EventSink receives captured IO events.
type EventSink func(evt ioevent.IOEvent)

// NetworkTap records HTTP traffic from Node CDP without a reverse proxy.
type NetworkTap struct {
	inspectPort int
	comm        string
	sink        EventSink
	conn        *websocket.Conn
	mu          sync.Mutex
	nextID      atomic.Int64
	nextFD      atomic.Int64
}

// StartNetworkTap connects to Node inspector and streams Network events until ctx is done.
func StartNetworkTap(ctx context.Context, inspectPort int, comm string, sink EventSink) (*NetworkTap, error) {
	if sink == nil {
		return nil, fmt.Errorf("sink required")
	}
	if inspectPort <= 0 {
		inspectPort = 9229
	}
	tap := &NetworkTap{
		inspectPort: inspectPort,
		comm:        comm,
		sink:        sink,
	}
	if err := tap.connect(ctx); err != nil {
		return nil, err
	}
	go tap.pump(ctx)
	return tap, nil
}

func (t *NetworkTap) connect(ctx context.Context) error {
	deadline := time.Now().Add(20 * time.Second)
	var wsURL string
	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		resp, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d/json/list", t.inspectPort))
		if err == nil {
			var targets []struct {
				WebSocketDebuggerURL string `json:"webSocketDebuggerUrl"`
				Type                 string `json:"type"`
			}
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			_ = json.Unmarshal(body, &targets)
			for _, target := range targets {
				if target.WebSocketDebuggerURL != "" {
					wsURL = target.WebSocketDebuggerURL
					break
				}
			}
		}
		if wsURL != "" {
			break
		}
		time.Sleep(300 * time.Millisecond)
	}
	if wsURL == "" {
		return fmt.Errorf("node inspector not available on port %d", t.inspectPort)
	}
	conn, err := websocket.Dial(wsURL, "", "http://localhost")
	if err != nil {
		return err
	}
	t.conn = conn
	if err := t.send("Network.enable", map[string]interface{}{}); err != nil {
		_ = conn.Close()
		return err
	}
	return nil
}

func (t *NetworkTap) pump(ctx context.Context) {
	defer t.Close()
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		var msg map[string]interface{}
		if err := websocket.JSON.Receive(t.conn, &msg); err != nil {
			return
		}
		method, _ := msg["method"].(string)
		params, _ := msg["params"].(map[string]interface{})
		switch method {
		case "Network.requestWillBeSent":
			t.handleRequest(params)
		case "Network.responseReceived":
			t.handleResponse(params)
		}
	}
}

func (t *NetworkTap) handleRequest(params map[string]interface{}) {
	req, _ := params["request"].(map[string]interface{})
	if req == nil {
		return
	}
	method, _ := req["method"].(string)
	url, _ := req["url"].(string)
	if method == "" || url == "" {
		return
	}
	path := url
	if strings.HasPrefix(url, "http") {
		if idx := strings.Index(url[8:], "/"); idx >= 0 {
			path = url[8+idx:]
		}
	}
	payload := fmt.Sprintf("%s %s HTTP/1.1\r\nHost: localhost\r\n\r\n", method, path)
	t.emit(1, []byte(payload))
}

func (t *NetworkTap) handleResponse(params map[string]interface{}) {
	resp, _ := params["response"].(map[string]interface{})
	if resp == nil {
		return
	}
	status, _ := resp["status"].(float64)
	statusText, _ := resp["statusText"].(string)
	url, _ := resp["url"].(string)
	if statusText == "" {
		statusText = httpStatusText(int(status))
	}
	payload := fmt.Sprintf("HTTP/1.1 %d %s\r\nX-BugIT-URL: %s\r\n\r\n", int(status), statusText, url)
	t.emit(0, []byte(payload))
}

func (t *NetworkTap) emit(isWrite uint8, payload []byte) {
	if len(payload) > ioevent.MaxPayloadLen {
		payload = payload[:ioevent.MaxPayloadLen]
	}
	var evt ioevent.IOEvent
	evt.TimestampNs = uint64(time.Now().UnixNano())
	evt.Fd = uint32(t.nextFD.Add(1))
	evt.IsWrite = isWrite
	evt.PayloadLen = uint32(len(payload))
	copy(evt.Payload[:], payload)
	name := t.comm
	if len(name) > ioevent.CommLen {
		name = name[:ioevent.CommLen]
	}
	copy(evt.Comm[:], []byte(name))
	t.sink(evt)
}

func (t *NetworkTap) send(method string, params map[string]interface{}) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.conn == nil {
		return fmt.Errorf("not connected")
	}
	id := int(t.nextID.Add(1))
	msg := map[string]interface{}{"id": id, "method": method, "params": params}
	return websocket.JSON.Send(t.conn, msg)
}

func (t *NetworkTap) Close() {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.conn != nil {
		_ = t.conn.Close()
		t.conn = nil
	}
}

func httpStatusText(code int) string {
	if text := http.StatusText(code); text != "" {
		return text
	}
	return "Unknown"
}
