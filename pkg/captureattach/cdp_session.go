package captureattach

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/net/websocket"
)

const bindingName = "bugitCapture"

// InspectTarget describes a Node inspector debug target.
type InspectTarget struct {
	ID                   string `json:"id"`
	Type                 string `json:"type"`
	Title                string `json:"title"`
	URL                  string `json:"url"`
	Description          string `json:"description"`
	WebSocketDebuggerURL string `json:"webSocketDebuggerUrl"`
}

// TargetPickOptions guides inspector target selection.
type TargetPickOptions struct {
	BackendPort int
	BackendPID  int
	CaptureRoot string
}

// CDPSession is a shared Chrome DevTools Protocol WebSocket session.
type CDPSession struct {
	conn      *websocket.Conn
	mu        sync.Mutex
	nextID    atomic.Int64
	pending   sync.Map
	handlers  map[string]func(map[string]interface{})
	handlerMu sync.RWMutex
	closed    atomic.Bool
}

// ListInspectTargets returns debugger targets on an inspector port.
func ListInspectTargets(port int) ([]InspectTarget, error) {
	if port <= 0 {
		return nil, fmt.Errorf("invalid inspector port %d", port)
	}
	client := &http.Client{Timeout: 800 * time.Millisecond}
	resp, err := client.Get(fmt.Sprintf("http://127.0.0.1:%d/json/list", port))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("inspector HTTP %d on port %d", resp.StatusCode, port)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var targets []InspectTarget
	if err := json.Unmarshal(body, &targets); err != nil {
		return nil, err
	}
	return targets, nil
}

// PickInspectTarget chooses the best WebSocket URL for backend attach.
func PickInspectTarget(targets []InspectTarget, opts TargetPickOptions) string {
	bestScore := -1
	var bestURL string
	portStr := strconv.Itoa(opts.BackendPort)
	pidStr := strconv.Itoa(opts.BackendPID)
	rootBase := strings.ToLower(filepath.Base(opts.CaptureRoot))

	for _, t := range targets {
		if t.WebSocketDebuggerURL == "" {
			continue
		}
		score := 0
		title := strings.ToLower(t.Title)
		url := strings.ToLower(t.URL)
		desc := strings.ToLower(t.Description)

		if t.Type == "node" || t.Type == "" {
			score += 10
		}
		if opts.BackendPID > 0 && (strings.Contains(title, pidStr) || strings.Contains(desc, pidStr)) {
			score += 25
		}
		if opts.BackendPort > 0 && (strings.Contains(title, portStr) || strings.Contains(url, portStr) || strings.Contains(desc, portStr)) {
			score += 8
		}
		if rootBase != "" && rootBase != "." && (strings.Contains(title, rootBase) || strings.Contains(url, rootBase)) {
			score += 5
		}
		if strings.Contains(title, "server") || strings.Contains(url, "server") {
			score += 2
		}
		if score > bestScore {
			bestScore = score
			bestURL = t.WebSocketDebuggerURL
		}
	}

	if bestURL != "" {
		return bestURL
	}
	for _, t := range targets {
		if t.WebSocketDebuggerURL != "" {
			return t.WebSocketDebuggerURL
		}
	}
	return ""
}

// ScoreInspectPort ranks how well an inspector port matches the backend process.
func ScoreInspectPort(port int, opts TargetPickOptions) int {
	targets, err := ListInspectTargets(port)
	if err != nil || len(targets) == 0 {
		return -1
	}
	best := -1
	portStr := strconv.Itoa(opts.BackendPort)
	pidStr := strconv.Itoa(opts.BackendPID)
	rootBase := strings.ToLower(filepath.Base(opts.CaptureRoot))
	for _, t := range targets {
		if t.WebSocketDebuggerURL == "" {
			continue
		}
		score := 1
		title := strings.ToLower(t.Title)
		if t.Type == "node" || t.Type == "" {
			score += 5
		}
		if opts.BackendPID > 0 && strings.Contains(title, pidStr) {
			score += 20
		}
		if opts.BackendPort > 0 && strings.Contains(title, portStr) {
			score += 5
		}
		if rootBase != "" && strings.Contains(title, rootBase) {
			score += 3
		}
		if score > best {
			best = score
		}
	}
	return best
}

// ConnectCDP dials the Node inspector and starts the event pump.
func ConnectCDP(ctx context.Context, inspectPort int, opts TargetPickOptions) (*CDPSession, error) {
	if inspectPort <= 0 {
		inspectPort = 9229
	}
	deadline := time.Now().Add(20 * time.Second)
	var wsURL string
	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}
		targets, err := ListInspectTargets(inspectPort)
		if err == nil {
			wsURL = PickInspectTarget(targets, opts)
		}
		if wsURL != "" {
			break
		}
		time.Sleep(300 * time.Millisecond)
	}
	if wsURL == "" {
		return nil, fmt.Errorf("node inspector not available on port %d", inspectPort)
	}

	conn, err := websocket.Dial(wsURL, "", "http://localhost")
	if err != nil {
		return nil, err
	}

	s := &CDPSession{
		conn:     conn,
		handlers: make(map[string]func(map[string]interface{})),
	}
	go s.pump()
	return s, nil
}

func (s *CDPSession) On(method string, handler func(map[string]interface{})) {
	s.handlerMu.Lock()
	defer s.handlerMu.Unlock()
	s.handlers[method] = handler
}

func (s *CDPSession) Call(ctx context.Context, method string, params map[string]interface{}) (map[string]interface{}, error) {
	if s.closed.Load() {
		return nil, fmt.Errorf("cdp session closed")
	}
	id := s.nextID.Add(1)
	ch := make(chan map[string]interface{}, 1)
	s.pending.Store(id, ch)
	defer s.pending.Delete(id)

	s.mu.Lock()
	msg := map[string]interface{}{"id": id, "method": method}
	if params != nil {
		msg["params"] = params
	}
	err := websocket.JSON.Send(s.conn, msg)
	s.mu.Unlock()
	if err != nil {
		return nil, err
	}

	select {
	case resp := <-ch:
		if errObj, ok := resp["error"].(map[string]interface{}); ok {
			return nil, fmt.Errorf("cdp error: %v", errObj)
		}
		if result, ok := resp["result"].(map[string]interface{}); ok {
			return result, nil
		}
		return map[string]interface{}{}, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (s *CDPSession) pump() {
	for {
		if s.closed.Load() {
			return
		}
		var msg map[string]interface{}
		if err := websocket.JSON.Receive(s.conn, &msg); err != nil {
			s.Close()
			return
		}
		if idVal, ok := msg["id"].(float64); ok {
			id := int64(idVal)
			if chRaw, ok := s.pending.Load(id); ok {
				ch := chRaw.(chan map[string]interface{})
				select {
				case ch <- msg:
				default:
				}
			}
			continue
		}
		method, _ := msg["method"].(string)
		if method == "" {
			continue
		}
		params, _ := msg["params"].(map[string]interface{})
		s.handlerMu.RLock()
		handler := s.handlers[method]
		s.handlerMu.RUnlock()
		if handler != nil {
			handler(params)
		}
	}
}

func (s *CDPSession) Close() {
	if s.closed.Swap(true) {
		return
	}
	s.mu.Lock()
	if s.conn != nil {
		_ = s.conn.Close()
		s.conn = nil
	}
	s.mu.Unlock()
}
