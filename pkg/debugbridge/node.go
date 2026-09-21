package debugbridge

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/bugit/dre-engine/api/manifest"
	"golang.org/x/net/websocket"
)

type NodeBridge struct {
	port   int
	conn   *websocket.Conn
	mu     sync.Mutex
	nextID int
}

func NewNodeBridge(port int) *NodeBridge {
	if port == 0 {
		port = 9229
	}
	return &NodeBridge{port: port}
}

func (n *NodeBridge) Connect(ctx context.Context) error {
	deadline := time.Now().Add(15 * time.Second)
	var wsURL string
	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		resp, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d/json/list", n.port))
		if err == nil {
			var targets []struct {
				WebSocketDebuggerURL string `json:"webSocketDebuggerUrl"`
			}
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			_ = json.Unmarshal(body, &targets)
			for _, t := range targets {
				if t.WebSocketDebuggerURL != "" {
					wsURL = t.WebSocketDebuggerURL
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
		return fmt.Errorf("node inspector not available on port %d", n.port)
	}
	conn, err := websocket.Dial(wsURL, "", "http://localhost")
	if err != nil {
		return err
	}
	n.conn = conn
	return n.send("Runtime.enable", nil)
}

func (n *NodeBridge) TopFrame() (*manifest.SourceRef, error) {
	n.mu.Lock()
	defer n.mu.Unlock()
	if n.conn == nil {
		return nil, fmt.Errorf("not connected")
	}
	resp, err := n.call("Runtime.evaluate", map[string]interface{}{
		"expression": "(new Error()).stack",
		"returnByValue": true,
	})
	if err != nil {
		return nil, err
	}
	stack, _ := resp["result"].(map[string]interface{})["value"].(string)
	return parseNodeStack(stack), nil
}

func parseNodeStack(stack string) *manifest.SourceRef {
	lines := strings.Split(stack, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "at ") && strings.Contains(line, ":") {
			// at fn (file:line:col) or at file:line:col
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
			parts := strings.Split(loc, ":")
			if len(parts) < 2 {
				continue
			}
			file := strings.Join(parts[:len(parts)-2], ":")
			if file == "" {
				file = parts[0]
			}
			lineNo, col := 0, 0
			fmt.Sscanf(parts[len(parts)-2], "%d", &lineNo)
			if len(parts) >= 3 {
				fmt.Sscanf(parts[len(parts)-1], "%d", &col)
			}
			return &manifest.SourceRef{File: file, Line: lineNo, Column: col, Function: fn}
		}
	}
	return nil
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
	_, err := n.call(method, params)
	return err
}

func (n *NodeBridge) call(method string, params map[string]interface{}) (map[string]interface{}, error) {
	n.nextID++
	id := n.nextID
	msg := map[string]interface{}{"id": id, "method": method, "params": params}
	if err := websocket.JSON.Send(n.conn, msg); err != nil {
		return nil, err
	}
	for {
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
