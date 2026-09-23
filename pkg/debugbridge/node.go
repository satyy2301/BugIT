package debugbridge

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/bugit/dre-engine/api/manifest"
	"github.com/bugit/dre-engine/pkg/captureattach"
	"golang.org/x/net/websocket"
)

type NodeBridge struct {
	port   int
	wsURL  string
	pick   captureattach.TargetPickOptions
	conn   *websocket.Conn
	mu     sync.Mutex
	nextID int
}

func NewNodeBridge(port int, pick captureattach.TargetPickOptions) *NodeBridge {
	if port == 0 {
		port = 9229
	}
	return &NodeBridge{port: port, pick: pick}
}

func NewNodeBridgeWS(wsURL string) *NodeBridge {
	return &NodeBridge{wsURL: wsURL}
}

func (n *NodeBridge) Connect(ctx context.Context) error {
	if n.wsURL != "" {
		return n.connectWS(ctx, n.wsURL)
	}
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		if insp, err := captureattach.ResolveBackendInspector(n.pick); err == nil && insp.WebSocketURL != "" {
			return n.connectWS(ctx, insp.WebSocketURL)
		}
		for port := captureattach.InspectorPortMin; port <= captureattach.InspectorPortMax; port++ {
			targets, err := captureattach.ListInspectTargets(port)
			if err != nil {
				continue
			}
			if ws := captureattach.PickInspectTarget(targets, n.pick); ws != "" {
				return n.connectWS(ctx, ws)
			}
		}
		time.Sleep(300 * time.Millisecond)
	}
	return fmt.Errorf("node inspector not available on port %d", n.port)
}

func (n *NodeBridge) connectWS(ctx context.Context, wsURL string) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	n.mu.Lock()
	defer n.mu.Unlock()
	conn, err := websocket.Dial(wsURL, "", "http://localhost")
	if err != nil {
		return err
	}
	n.conn = conn
	return n.sendLocked("Runtime.enable", nil)
}

func (n *NodeBridge) TopFrame() (*manifest.SourceRef, error) {
	n.mu.Lock()
	defer n.mu.Unlock()
	if n.conn == nil {
		return nil, fmt.Errorf("not connected")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	resp, err := n.callLocked(ctx, "Runtime.evaluate", map[string]interface{}{
		"expression":    "(new Error()).stack",
		"returnByValue": true,
	})
	if err != nil {
		return nil, err
	}
	stack, ok := resp["value"].(string)
	if !ok || stack == "" {
		return nil, nil
	}
	return parseNodeStack(stack), nil
}

func parseNodeStack(stack string) *manifest.SourceRef {
	lines := strings.Split(stack, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "at ") && strings.Contains(line, ":") {
			start := strings.Index(line, "(")
			end := strings.LastIndex(line, ")")
			loc := line
			fn := ""
			if start >= 0 && end > start {
				fn = strings.TrimSpace(line[3:start])
				loc = line[start+1 : end]
			} else {
				loc = strings.TrimSpace(strings.TrimPrefix(line, "at "))
			}
			file, lineNo, col := parseStackLocation(loc)
			if file == "" {
				continue
			}
			return &manifest.SourceRef{File: file, Line: lineNo, Column: col, Function: fn}
		}
	}
	return nil
}

func parseStackLocation(loc string) (file string, line int, col int) {
	lastColon := strings.LastIndex(loc, ":")
	if lastColon < 0 {
		return loc, 0, 0
	}
	colPart := loc[lastColon+1:]
	rest := loc[:lastColon]
	lineColon := strings.LastIndex(rest, ":")
	if lineColon < 0 {
		return loc, 0, 0
	}
	linePart := rest[lineColon+1:]
	file = rest[:lineColon]
	fmt.Sscanf(linePart, "%d", &line)
	fmt.Sscanf(colPart, "%d", &col)
	return file, line, col
}

func (n *NodeBridge) Close() {
	n.mu.Lock()
	defer n.mu.Unlock()
	if n.conn != nil {
		_ = n.conn.Close()
		n.conn = nil
	}
}

func (n *NodeBridge) send(method string, params map[string]interface{}) error {
	n.mu.Lock()
	defer n.mu.Unlock()
	return n.sendLocked(method, params)
}

func (n *NodeBridge) sendLocked(method string, params map[string]interface{}) error {
	_, err := n.callLocked(context.Background(), method, params)
	return err
}

func (n *NodeBridge) call(method string, params map[string]interface{}) (map[string]interface{}, error) {
	n.mu.Lock()
	defer n.mu.Unlock()
	return n.callLocked(context.Background(), method, params)
}

func (n *NodeBridge) callLocked(ctx context.Context, method string, params map[string]interface{}) (map[string]interface{}, error) {
	if n.conn == nil {
		return nil, fmt.Errorf("not connected")
	}
	n.nextID++
	id := n.nextID
	msg := map[string]interface{}{"id": id, "method": method, "params": params}
	if err := websocket.JSON.Send(n.conn, msg); err != nil {
		return nil, err
	}
	deadline := time.Now().Add(15 * time.Second)
	for {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("cdp call timeout for %s", method)
		}
		var resp map[string]interface{}
		if err := websocket.JSON.Receive(n.conn, &resp); err != nil {
			return nil, err
		}
		if respID, ok := resp["id"].(float64); ok && int(respID) == id {
			if errObj, ok := resp["error"].(map[string]interface{}); ok {
				return nil, fmt.Errorf("cdp error: %v", errObj)
			}
			if result, ok := resp["result"].(map[string]interface{}); ok {
				return result, nil
			}
			return map[string]interface{}{}, nil
		}
	}
}
