package summary

import (
	"strings"

	"github.com/bugit/dre-engine/api/ioevent"
)

const maxPayloadBytes = 2048

type EventSummary struct {
	Index          int    `json:"index"`
	Service        string `json:"service"`
	Direction      string `json:"direction"`
	Summary        string `json:"summary"`
	PayloadPreview string `json:"payload_preview"`
	Payload        string `json:"payload"`
	IsError        bool   `json:"is_error"`
	TimestampNs    uint64 `json:"timestamp_ns"`
}

func SummarizeEvents(events []ioevent.IOEvent) []EventSummary {
	out := make([]EventSummary, 0, len(events))
	for i, e := range events {
		if e.IsWrite == 2 {
			continue // skip sched_switch markers in timeline
		}
		pl := int(e.PayloadLen)
		if pl > len(e.Payload) {
			pl = len(e.Payload)
		}
		payload := string(e.Payload[:pl])
		svc := strings.TrimRight(string(e.Comm[:]), "\x00")
		svc = strings.TrimSpace(svc)
		if svc == "" {
			svc = "unknown"
		}
		dir := "out"
		if e.IsWrite == 0 {
			dir = "in"
		}
		sum, isErr := httpSummary(payload)
		preview := payload
		if len(preview) > 120 {
			preview = preview[:120] + "..."
		}
		preview = strings.ReplaceAll(preview, "\r\n", " ")
		full := payload
		if len(full) > maxPayloadBytes {
			full = full[:maxPayloadBytes] + "..."
		}
		out = append(out, EventSummary{
			Index:          i,
			Service:        svc,
			Direction:      dir,
			Summary:        sum,
			PayloadPreview: preview,
			Payload:        full,
			IsError:        isErr,
			TimestampNs:    e.TimestampNs,
		})
	}
	return out
}

func httpSummary(payload string) (string, bool) {
	if payload == "" {
		return "(empty)", false
	}
	lines := strings.Split(payload, "\r\n")
	if len(lines) == 0 {
		lines = strings.Split(payload, "\n")
	}
	first := lines[0]
	isErr := strings.Contains(first, " 500 ") || strings.Contains(first, " 502 ") ||
		strings.Contains(first, " 503 ") || strings.Contains(first, " 5")

	if strings.HasPrefix(first, "HTTP/1.") {
		return first, isErr
	}
	parts := strings.Fields(first)
	if len(parts) >= 2 && (parts[0] == "GET" || parts[0] == "POST" || parts[0] == "PUT" || parts[0] == "DELETE") {
		return parts[0] + " " + parts[1], isErr
	}
	if len(first) > 60 {
		return first[:60] + "...", isErr
	}
	return first, isErr
}

func FlowDescription(events []ioevent.IOEvent) string {
	parts := []string{}
	for _, e := range events {
		pl := int(e.PayloadLen)
		if pl > len(e.Payload) {
			pl = len(e.Payload)
		}
		payload := string(e.Payload[:pl])
		svc := strings.TrimSpace(strings.TrimRight(string(e.Comm[:]), "\x00"))
		sum, isErr := httpSummary(payload)
		if e.IsWrite == 2 || payload == "" {
			continue
		}
		label := svc + " " + sum
		if isErr {
			label = label + " [ERROR]"
		}
		parts = append(parts, label)
	}
	return strings.Join(parts, " → ")
}
