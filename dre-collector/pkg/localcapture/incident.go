package localcapture

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/bugit/dre-engine/api/ioevent"
	"github.com/bugit/dre-engine/api/manifest"
)

// BuildIncident derives human-readable incident metadata from captured events.
func BuildIncident(events []ioevent.IOEvent) *manifest.Incident {
	if len(events) == 0 {
		return nil
	}

	var failedStep string
	var rootCause string
	var title string
	services := map[string]bool{}

	for _, ev := range events {
		payload := string(ev.Payload[:ev.PayloadLen])
		comm := strings.TrimRight(string(ev.Comm[:]), "\x00")
		if comm != "" {
			services[comm] = true
		}
		if ev.IsWrite != 0 {
			if method, path := parseHTTPRequestLine(payload); method != "" {
				services[method+" "+path] = true
			}
			continue
		}
		if status, detail := parseHTTPStatus(payload); status >= 400 {
			step := summarizePayload(payload)
			if failedStep == "" || status >= 500 {
				failedStep = step
				rootCause = detail
				if status >= 500 {
					title = fmt.Sprintf("Server error: HTTP %d", status)
				} else {
					title = fmt.Sprintf("Client error: HTTP %d", status)
				}
			}
		}
	}

	if title == "" {
		title = "Captured incident"
	}
	if rootCause == "" {
		rootCause = "See event timeline for failing request/response"
	}
	if failedStep == "" {
		failedStep = "See highlighted error event in timeline"
	}

	var svcList []string
	for s := range services {
		if strings.Contains(s, " ") {
			continue
		}
		svcList = append(svcList, s)
	}

	return &manifest.Incident{
		Title:      title,
		Summary:    fmt.Sprintf("Local capture with %d events. %s", len(events), failedStep),
		RootCause:  rootCause,
		Services:   svcList,
		FailedStep: failedStep,
	}
}

func parseHTTPRequestLine(payload string) (method, path string) {
	line, _, _ := strings.Cut(payload, "\r\n")
	parts := strings.Fields(line)
	if len(parts) < 2 {
		return "", ""
	}
	return parts[0], parts[1]
}

func parseHTTPStatus(payload string) (int, string) {
	line, rest, ok := strings.Cut(payload, "\r\n")
	if !ok {
		return 0, ""
	}
	if !strings.HasPrefix(line, "HTTP/") {
		return 0, ""
	}
	parts := strings.Fields(line)
	if len(parts) < 2 {
		return 0, ""
	}
	status := 0
	fmt.Sscanf(parts[1], "%d", &status)
	body, _, _ := strings.Cut(rest, "\r\n\r\n")
	return status, strings.TrimSpace(body)
}

func summarizePayload(payload string) string {
	line, _, _ := strings.Cut(payload, "\r\n")
	if len(line) > 120 {
		return line[:120] + "..."
	}
	return line
}

func parseHTTPHost(payload string) string {
	for _, line := range bytes.Split([]byte(payload), []byte("\r\n")) {
		if bytes.HasPrefix(bytes.ToLower(line), []byte("host:")) {
			return strings.TrimSpace(string(line[5:]))
		}
	}
	return ""
}
