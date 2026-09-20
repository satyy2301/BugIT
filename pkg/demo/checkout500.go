package demo

import (
	"time"

	"github.com/bugit/dre-engine/api/ioevent"
	"github.com/bugit/dre-engine/api/manifest"
)

// Checkout500 returns a realistic payment-failure incident for demos.
func Checkout500() ([]ioevent.IOEvent, manifest.VectorGraph, manifest.RedactionLog, manifest.Manifest) {
	base := uint64(time.Now().UnixNano())
	events := []ioevent.IOEvent{
		makeEvent(base+1e6, "api-gatewa", 4, 0, "GET /health HTTP/1.1\r\nHost: api-gateway\r\n\r\n"),
		makeEvent(base+2e6, "order-api", 7, 0, "POST /checkout HTTP/1.1\r\nHost: order-api\r\nContent-Type: application/json\r\nAuthorization: Bearer eyJhbGciOiJIUzI1NiJ9.demo\r\n\r\n{\"order_id\":\"ORD-8842\",\"amount\":49.99}"),
		makeEvent(base+3e6, "order-api", 8, 1, "POST /charge HTTP/1.1\r\nHost: payment-service\r\nX-DRE-Vector-Clock: node-a:1\r\nContent-Type: application/json\r\n\r\n{\"order_id\":\"ORD-8842\",\"card\":\"****4242\"}"),
		makeEvent(base+4e6, "payment-svc", 9, 0, "POST /charge HTTP/1.1\r\nHost: payment-service\r\n\r\n{\"order_id\":\"ORD-8842\"}"),
		makeEvent(base+5e6, "payment-svc", 9, 1, "HTTP/1.1 500 Internal Server Error\r\nContent-Type: application/json\r\n\r\n{\"error\":\"card_processor_down\"}"),
		makeEvent(base+6e6, "order-api", 8, 0, "HTTP/1.1 500 Internal Server Error\r\nContent-Type: application/json\r\n\r\n{\"error\":\"card_processor_down\"}"),
		makeEvent(base+7e6, "order-api", 7, 1, "HTTP/1.1 502 Bad Gateway\r\nContent-Type: application/json\r\n\r\n{\"error\":\"payment_unavailable\",\"order_id\":\"ORD-8842\"}"),
		makeEvent(base+8e6, "api-gatewa", 4, 1, "HTTP/1.1 502 Bad Gateway\r\nContent-Type: application/json\r\n\r\n{\"error\":\"checkout_failed\"}"),
		makeEvent(base+9e6, "order-api", 0, 2, ""),
	}

	graph := manifest.VectorGraph{
		Nodes: []manifest.VectorNode{
			{ID: "node-a:1", NodeID: "node-a", Sequence: 1, Timestamp: base + 3e6},
			{ID: "node-b:1", NodeID: "node-b", Sequence: 1, Timestamp: base + 4e6},
			{ID: "node-b:2", NodeID: "node-b", Sequence: 2, Timestamp: base + 5e6},
			{ID: "node-a:2", NodeID: "node-a", Sequence: 2, Timestamp: base + 6e6},
		},
		Edges: []manifest.VectorEdge{
			{From: "node-a:1", To: "node-b:1"},
			{From: "node-b:1", To: "node-b:2"},
			{From: "node-b:2", To: "node-a:2"},
		},
	}

	redact := manifest.RedactionLog{
		Entries: []manifest.RedactionEntry{
			{Offset: 120, Length: 32, Rule: `Bearer\s+[^\s]+`},
		},
	}

	m := manifest.Manifest{
		Cluster: "staging-eu-west",
		Nodes:   []string{"node-a", "node-b"},
		Trigger: manifest.Trigger{
			Type:   manifest.TriggerHTTP5xx,
			Detail: "payment-service returned 500 on POST /charge",
		},
		CapturedAt: time.Now().UTC(),
		Incident: &manifest.Incident{
			Title:      "Checkout failed: payment service returned 500",
			Summary:    "Customer checkout for order ORD-8842 failed when order-api called payment-service. Payment gateway was down; order-api returned 502 to the client.",
			RootCause:  "payment-service upstream card processor unreachable (card_processor_down)",
			Services:   []string{"order-api", "payment-service", "api-gateway"},
			FailedStep: "POST /charge on payment-service → HTTP 500",
		},
	}
	return events, graph, redact, m
}

func makeEvent(ts uint64, comm string, fd uint32, isWrite uint8, payload string) ioevent.IOEvent {
	var e ioevent.IOEvent
	e.PidTgid = 1000
	e.TimestampNs = ts
	e.Fd = fd
	e.IsWrite = isWrite
	copy(e.Comm[:], comm)
	if len(payload) > ioevent.MaxPayloadLen {
		payload = payload[:ioevent.MaxPayloadLen]
	}
	e.PayloadLen = uint32(len(payload))
	copy(e.Payload[:], payload)
	return e
}
