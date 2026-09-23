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

// MinBackendTargetScore is the minimum score required to attach to an inspector target.
const MinBackendTargetScore = 25

// MinNewTargetScore accepts a newly appeared inspector target on a non-default port.
const MinNewTargetScore = 15

// DefaultBackendInspectPort is used when :9229 is owned by another process (e.g. Next.js).
const DefaultBackendInspectPort = 9230

// InspectorPortMin/Max define the local Node inspector port scan range.
const (
	InspectorPortMin = 9229
	InspectorPortMax = 9239
)

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

// BackendInspector holds a resolved backend debug target.
type BackendInspector struct {
	Port         int
	WebSocketURL string
	Title        string
	URL          string
	Score        int
}

// PortTarget pairs an inspector port with a debug target entry.
type PortTarget struct {
	Port   int
	Target InspectTarget
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

func normalizePathForMatch(path string) string {
	path = strings.ToLower(strings.TrimSpace(path))
	path = strings.TrimPrefix(path, "file://")
	path = strings.ReplaceAll(path, "\\", "/")
	if len(path) >= 2 && path[1] == ':' {
		path = strings.ReplaceAll(path, ":", "")
	}
	return path
}

func isDisqualifiedTarget(title, url, desc string, frontendPort int) bool {
	joined := title + " " + url + " " + desc
	if strings.Contains(joined, "next.js") || strings.Contains(joined, "next dev") {
		return true
	}
	if strings.Contains(joined, "next/") && strings.Contains(joined, "node_modules") {
		return true
	}
	if frontendPort > 0 {
		portStr := ":" + strconv.Itoa(frontendPort)
		if strings.Contains(joined, portStr) && !strings.Contains(joined, "server.js") {
			return true
		}
	}
	if strings.Contains(title, "next") && strings.Contains(url, ".next") {
		return true
	}
	return false
}

// ScoreTarget ranks how well a debug target matches the backend process.
func ScoreTarget(t InspectTarget, opts TargetPickOptions) int {
	if t.WebSocketDebuggerURL == "" {
		return -1
	}
	title := strings.ToLower(t.Title)
	url := strings.ToLower(t.URL)
	desc := strings.ToLower(t.Description)
	frontendPort := 3000
	if opts.BackendPort == frontendPort {
		frontendPort = 0
	}
	if isDisqualifiedTarget(title, url, desc, frontendPort) {
		return -1
	}

	score := 0
	if t.Type == "node" || t.Type == "" {
		score += 10
	}

	rootNorm := normalizePathForMatch(opts.CaptureRoot)
	urlNorm := normalizePathForMatch(t.URL)
	if rootNorm != "" && urlNorm != "" && strings.Contains(urlNorm, rootNorm) {
		score += 40
	}

	if strings.HasSuffix(urlNorm, "server.js") || strings.Contains(urlNorm, "/server.js") {
		score += 15
	}
	if strings.Contains(title, "server.js") {
		score += 10
	}

	portStr := strconv.Itoa(opts.BackendPort)
	pidStr := strconv.Itoa(opts.BackendPID)
	if opts.BackendPID > 0 && (strings.Contains(title, pidStr) || strings.Contains(desc, pidStr)) {
		score += 25
	}
	if opts.BackendPort > 0 && (strings.Contains(title, portStr) || strings.Contains(url, portStr) || strings.Contains(desc, portStr)) {
		score += 8
	}

	rootBase := strings.ToLower(filepath.Base(opts.CaptureRoot))
	if rootBase != "" && rootBase != "." && (strings.Contains(title, rootBase) || strings.Contains(urlNorm, rootBase)) {
		score += 5
	}
	return score
}

// PickInspectTarget chooses the best WebSocket URL for backend attach.
func PickInspectTarget(targets []InspectTarget, opts TargetPickOptions) string {
	bestScore := -1
	var bestURL string
	for _, t := range targets {
		score := ScoreTarget(t, opts)
		if score > bestScore {
			bestScore = score
			bestURL = t.WebSocketDebuggerURL
		}
	}
	if bestURL != "" && bestScore >= MinBackendTargetScore {
		return bestURL
	}
	return ""
}

// PickInspectTargetInfo returns the best target with score metadata.
func PickInspectTargetInfo(targets []InspectTarget, opts TargetPickOptions) BackendInspector {
	best := BackendInspector{Score: -1}
	for _, t := range targets {
		score := ScoreTarget(t, opts)
		if score > best.Score {
			best = BackendInspector{
				WebSocketURL: t.WebSocketDebuggerURL,
				Title:        t.Title,
				URL:          t.URL,
				Score:        score,
			}
		}
	}
	return best
}

func targetIdentity(port int, t InspectTarget) string {
	if t.ID != "" {
		return strconv.Itoa(port) + ":" + t.ID
	}
	if t.WebSocketDebuggerURL != "" {
		return strconv.Itoa(port) + ":" + t.WebSocketDebuggerURL
	}
	return strconv.Itoa(port) + ":" + t.Title + ":" + t.URL
}

func isNewBackendTarget(port int, t InspectTarget, opts TargetPickOptions) bool {
	if port == InspectorPortMin {
		return false
	}
	rootNorm := normalizePathForMatch(opts.CaptureRoot)
	urlNorm := normalizePathForMatch(t.URL)
	if rootNorm != "" {
		if !strings.Contains(urlNorm, rootNorm) && !strings.Contains(urlNorm, filepath.Base(rootNorm)) {
			return false
		}
	}
	return ScoreTarget(t, opts) >= MinNewTargetScore
}

// SnapshotPortTargets lists all inspector targets across the scan range.
func SnapshotPortTargets() []PortTarget {
	var out []PortTarget
	for port := InspectorPortMin; port <= InspectorPortMax; port++ {
		targets, err := ListInspectTargets(port)
		if err != nil || len(targets) == 0 {
			continue
		}
		for _, t := range targets {
			out = append(out, PortTarget{Port: port, Target: t})
		}
	}
	return out
}

// DiffNewTargets returns targets present in after but not in before.
func DiffNewTargets(before, after []PortTarget) []PortTarget {
	seen := make(map[string]struct{}, len(before))
	for _, pt := range before {
		seen[targetIdentity(pt.Port, pt.Target)] = struct{}{}
	}
	var out []PortTarget
	for _, pt := range after {
		if _, ok := seen[targetIdentity(pt.Port, pt.Target)]; ok {
			continue
		}
		out = append(out, pt)
	}
	return out
}

// ResolveBackendInspectorWithDiff prefers newly appeared backend targets on non-9229 ports.
func ResolveBackendInspectorWithDiff(before []PortTarget, opts TargetPickOptions) (BackendInspector, error) {
	after := SnapshotPortTargets()
	newTargets := DiffNewTargets(before, after)

	bestNew := BackendInspector{Score: -1}
	for _, pt := range newTargets {
		if !isNewBackendTarget(pt.Port, pt.Target, opts) {
			continue
		}
		score := ScoreTarget(pt.Target, opts)
		if score > bestNew.Score {
			bestNew = BackendInspector{
				Port:         pt.Port,
				WebSocketURL: pt.Target.WebSocketDebuggerURL,
				Title:        pt.Target.Title,
				URL:          pt.Target.URL,
				Score:        score,
			}
		}
	}
	if bestNew.WebSocketURL != "" && bestNew.Score >= MinNewTargetScore {
		return bestNew, nil
	}
	return ResolveBackendInspector(opts)
}

// FindFreeInspectorPort returns the first usable local inspector port.
func FindFreeInspectorPort() int {
	if !InspectAvailable(InspectorPortMin) {
		return InspectorPortMin
	}
	for port := DefaultBackendInspectPort; port <= InspectorPortMax; port++ {
		if !InspectAvailable(port) {
			return port
		}
	}
	return DefaultBackendInspectPort
}

// ResolveBackendInspector scans inspector ports and picks the backend debug target.
func ResolveBackendInspector(opts TargetPickOptions) (BackendInspector, error) {
	best := BackendInspector{Score: -1}
	for port := InspectorPortMin; port <= InspectorPortMax; port++ {
		targets, err := ListInspectTargets(port)
		if err != nil {
			continue
		}
		for _, t := range targets {
			score := ScoreTarget(t, opts)
			if score > best.Score {
				best = BackendInspector{
					Port:         port,
					WebSocketURL: t.WebSocketDebuggerURL,
					Title:        t.Title,
					URL:          t.URL,
					Score:        score,
				}
			}
		}
	}
	if best.WebSocketURL == "" || best.Score < MinBackendTargetScore {
		return BackendInspector{}, fmt.Errorf("no backend inspector target found (best score %d, need %d) — ensure backend is Node >=18", best.Score, MinBackendTargetScore)
	}
	return best, nil
}

// ScoreInspectPort ranks how well an inspector port matches the backend process.
func ScoreInspectPort(port int, opts TargetPickOptions) int {
	targets, err := ListInspectTargets(port)
	if err != nil || len(targets) == 0 {
		return -1
	}
	best := PickInspectTargetInfo(targets, opts)
	return best.Score
}

// ListAllInspectTargets returns targets across the inspector port range for diagnostics.
func ListAllInspectTargets() []struct {
	Port    int
	Targets []InspectTarget
} {
	var out []struct {
		Port    int
		Targets []InspectTarget
	}
	for port := InspectorPortMin; port <= InspectorPortMax; port++ {
		targets, err := ListInspectTargets(port)
		if err != nil || len(targets) == 0 {
			continue
		}
		out = append(out, struct {
			Port    int
			Targets []InspectTarget
		}{Port: port, Targets: targets})
	}
	return out
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
	return ConnectCDPURL(ctx, wsURL)
}

// ConnectCDPURL dials a specific inspector WebSocket URL.
func ConnectCDPURL(ctx context.Context, wsURL string) (*CDPSession, error) {
	if wsURL == "" {
		return nil, fmt.Errorf("empty inspector websocket url")
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
				ch <- msg
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
