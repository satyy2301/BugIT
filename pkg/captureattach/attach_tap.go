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
	InspectPort int
	BackendPort int
	BackendPID  int
	CaptureRoot string
	Comm        string
	Sink        EventSink
}

// AttachTap captures inbound HTTP via diagnostics_channel and outbound via CDP Network.
type AttachTap struct {
	session      *CDPSession
	sink         EventSink
	comm         string
	nextFD       atomic.Int64
	inboundCount atomic.Int64
}

// StartAttachTap connects to Node inspector and starts hybrid HTTP capture.
func StartAttachTap(ctx context.Context, opts AttachOptions) (*AttachTap, error) {
	if opts.Sink == nil {
		return nil, fmt.Errorf("sink required")
	}
	if opts.InspectPort <= 0 {
		opts.InspectPort = 9229
	}
	comm := opts.Comm
	if comm == "" {
		comm = "node"
	}

	pick := TargetPickOptions{
		BackendPort: opts.BackendPort,
		BackendPID:  opts.BackendPID,
		CaptureRoot: opts.CaptureRoot,
	}

	session, err := ConnectCDP(ctx, opts.InspectPort, pick)
	if err != nil {
		return nil, err
	}

	tap := &AttachTap{
		session: session,
		sink:    opts.Sink,
		comm:    comm,
	}

	tap.registerHandlers()
	if err := tap.setup(ctx); err != nil {
		session.Close()
		return nil, err
	}

	go func() {
		<-ctx.Done()
		session.Close()
	}()

	return tap, nil
}

func (t *AttachTap) registerHandlers() {
	t.session.On("Network.requestWillBeSent", t.handleOutboundRequest)
	t.session.On("Network.responseReceived", t.handleOutboundResponse)
	t.session.On("Runtime.bindingCalled", t.handleBindingCalled)
}

func (t *AttachTap) setup(ctx context.Context) error {
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
	if ok := evalBool(result, "result", "value"); !ok {
		return fmt.Errorf("server hook inject returned false — ensure Node >=18 with diagnostics_channel")
	}
	return nil
}

func evalBool(result map[string]interface{}, keys ...string) bool {
	cur := interface{}(result)
	for _, k := range keys {
		m, ok := cur.(map[string]interface{})
		if !ok {
			return false
		}
		cur = m[k]
	}
	v, ok := cur.(bool)
	return ok && v
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
	}
	if err := json.Unmarshal([]byte(payloadStr), &msg); err != nil {
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
	path := url
	if strings.HasPrefix(url, "http") {
		if idx := strings.Index(url[8:], "/"); idx >= 0 {
			path = url[8+idx:]
		}
	}
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
