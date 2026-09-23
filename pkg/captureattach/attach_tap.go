package captureattach

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync/atomic"
	"time"

	"github.com/bugit/dre-engine/api/ioevent"
)

//go:embed inject/server_hook.js
var serverHookJS string

// AttachOptions configures hybrid attach capture (inbound server + outbound network).
type AttachOptions struct {
	InspectPort   int
	InspectorWSURL string
	InspectorTarget BackendInspector
	BackendPort   int
	BackendPID    int
	CaptureRoot   string
	Comm           string
	Sink           EventSink
	DisablePreload bool
}

// AttachTap captures inbound HTTP via diagnostics_channel and outbound via CDP Network.
type AttachTap struct {
	session      *CDPSession
	sink         EventSink
	comm         string
	target       BackendInspector
	nextFD       atomic.Int64
	inboundCount atomic.Int64
	pingSeen     atomic.Bool
}

// StartAttachTap connects to Node inspector and starts hybrid HTTP capture.
func StartAttachTap(ctx context.Context, opts AttachOptions) (*AttachTap, error) {
	if opts.Sink == nil {
		return nil, fmt.Errorf("sink required")
	}
	comm := opts.Comm
	if comm == "" {
		comm = "node"
	}

	var session *CDPSession
	var err error
	if opts.InspectorWSURL != "" {
		session, err = ConnectCDPURL(ctx, opts.InspectorWSURL)
	} else {
		pick := TargetPickOptions{
			BackendPort: opts.BackendPort,
			BackendPID:  opts.BackendPID,
			CaptureRoot: opts.CaptureRoot,
		}
		port := opts.InspectPort
		if port <= 0 {
			port = 9229
		}
		session, err = ConnectCDP(ctx, port, pick)
	}
	if err != nil {
		return nil, err
	}

	tap := &AttachTap{
		session: session,
		sink:    opts.Sink,
		comm:    comm,
		target:  opts.InspectorTarget,
	}

	tap.registerHandlers()
	if err := tap.setup(ctx, opts); err != nil {
		session.Close()
		return nil, err
	}

	go func() {
		<-ctx.Done()
		session.Close()
	}()

	return tap, nil
}

func (t *AttachTap) Target() BackendInspector {
	return t.target
}

func (t *AttachTap) registerHandlers() {
	t.session.On("Network.requestWillBeSent", t.handleOutboundRequest)
	t.session.On("Network.responseReceived", t.handleOutboundResponse)
	t.session.On("Runtime.bindingCalled", t.handleBindingCalled)
}

func (t *AttachTap) setup(ctx context.Context, opts AttachOptions) error {
	if _, err := t.session.Call(ctx, "Runtime.enable", nil); err != nil {
		return fmt.Errorf("Runtime.enable: %w", err)
	}
	if _, err := t.session.Call(ctx, "Runtime.addBinding", map[string]interface{}{
		"name": bindingName,
	}); err != nil {
		return fmt.Errorf("Runtime.addBinding: %w", err)
	}
	if _, err := t.session.Call(ctx, "Network.enable", nil); err != nil {
		return fmt.Errorf("Network.enable: %w", err)
	}

	result, err := t.session.Call(ctx, "Runtime.evaluate", map[string]interface{}{
		"expression":    serverHookJS,
		"returnByValue": true,
	})
	if err != nil {
		return fmt.Errorf("server hook inject: %w", err)
	}
	if ok := evalRemoteBool(result); !ok {
		return fmt.Errorf("server hook inject returned false — ensure Node >=18 with diagnostics_channel on the backend process")
	}

	readyResult, err := t.session.Call(ctx, "Runtime.evaluate", map[string]interface{}{
		"expression":    "global.__bugitServerTap === true && !!(global.__bugitServerTapReady && global.__bugitServerTapReady.subscribed)",
		"returnByValue": true,
	})
	if err != nil {
		return fmt.Errorf("server hook ready check: %w", err)
	}
	if ok := evalRemoteBool(readyResult); !ok {
		return fmt.Errorf("attached to wrong Node process — hook not active; retry Record after backend inspector is enabled")
	}

	_, err = t.session.Call(ctx, "Runtime.evaluate", map[string]interface{}{
		"expression":    `typeof bugitCapture === 'function' && bugitCapture(JSON.stringify({ping:true,dir:0,payload:"BugIT attach ping"}))`,
		"returnByValue": true,
	})
	if err != nil {
		return fmt.Errorf("server hook ping: %w", err)
	}
	if err := t.waitPing(ctx, 500*time.Millisecond); err != nil {
		return fmt.Errorf("server hook binding not reachable: %w", err)
	}
	if opts.DisablePreload {
		_, err = t.session.Call(ctx, "Runtime.evaluate", map[string]interface{}{
			"expression": "global.__bugitPreloadDisabled = true",
		})
		if err != nil {
			return fmt.Errorf("disable preload capture: %w", err)
		}
	}
	return nil
}

func evalRemoteBool(result map[string]interface{}) bool {
	if v, ok := result["value"].(bool); ok {
		return v
	}
	if nested, ok := result["result"].(map[string]interface{}); ok {
		if v, ok := nested["value"].(bool); ok {
			return v
		}
	}
	return false
}

func (t *AttachTap) waitPing(ctx context.Context, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if t.pingSeen.Load() {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		time.Sleep(25 * time.Millisecond)
	}
	return fmt.Errorf("timeout waiting for binding ping")
}

func (t *AttachTap) InboundCount() int64 {
	return t.inboundCount.Load()
}

func (t *AttachTap) Close() {
	t.session.Close()
}

func (t *AttachTap) handleBindingCalled(params map[string]interface{}) {
	payloadStr, _ := params["payload"].(string)
	if payloadStr == "" {
		return
	}
	var msg struct {
		Dir     int    `json:"dir"`
		Payload string `json:"payload"`
		Ping    bool   `json:"ping"`
	}
	if err := json.Unmarshal([]byte(payloadStr), &msg); err != nil {
		return
	}
	if msg.Ping {
		t.pingSeen.Store(true)
		return
	}
	if msg.Payload == "" {
		return
	}
	isWrite := uint8(0)
	if msg.Dir == 1 {
		isWrite = 1
	}
	t.inboundCount.Add(1)
	t.emit(isWrite, []byte(msg.Payload))
}

func (t *AttachTap) handleOutboundRequest(params map[string]interface{}) {
	req, _ := params["request"].(map[string]interface{})
	if req == nil {
		return
	}
	method, _ := req["method"].(string)
	url, _ := req["url"].(string)
	if method == "" || url == "" {
		return
	}
	path := outboundURLPath(url)
	payload := fmt.Sprintf("%s %s HTTP/1.1\r\nHost: localhost\r\n\r\n", method, path)
	t.emit(1, []byte(payload))
}

func (t *AttachTap) handleOutboundResponse(params map[string]interface{}) {
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

func (t *AttachTap) emit(isWrite uint8, payload []byte) {
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

func outboundURLPath(url string) string {
	if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
		return url
	}
	schemeEnd := strings.Index(url, "://")
	if schemeEnd < 0 {
		return url
	}
	rest := url[schemeEnd+3:]
	if idx := strings.Index(rest, "/"); idx >= 0 {
		return rest[idx:]
	}
	return "/"
}

func httpStatusText(code int) string {
	if text := http.StatusText(code); text != "" {
		return text
	}
	return "Unknown"
}

// ParseBindingMessage converts a Runtime.bindingCalled payload to wire bytes and direction.
func ParseBindingMessage(payloadJSON string) (isWrite uint8, wire []byte, ok bool) {
	var msg struct {
		Dir     int    `json:"dir"`
		Payload string `json:"payload"`
	}
	if err := json.Unmarshal([]byte(payloadJSON), &msg); err != nil || msg.Payload == "" {
		return 0, nil, false
	}
	if msg.Dir == 1 {
		return 1, []byte(msg.Payload), true
	}
	return 0, []byte(msg.Payload), true
}
