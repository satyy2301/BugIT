package discover

import (
	"net"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/bugit/dre-engine/pkg/project"
)

// Result holds auto-discovered local services for attach-mode capture.
type Result struct {
	Workspace    string
	CaptureRoot  string
	BackendPort  int
	FrontendPort int
	BackendPID   int
	InspectPort  int
}

var apiURLRe = regexp.MustCompile(`(?i)(?:NEXT_PUBLIC_API_URL|API_URL|VITE_API_URL)\s*=\s*https?://[^:]+:(\d+)`)

// Discover scans the workspace for backend/frontend ports and live listeners.
func Discover(workspace string) Result {
	abs, _ := filepath.Abs(workspace)
	root := project.FindCaptureRoot(abs)
	res := Result{
		Workspace:   abs,
		CaptureRoot: root,
		BackendPort: project.DetectPublicPort(root),
	}

	for _, rel := range []string{".", "web", "frontend", "client", "apps/web"} {
		dir := rel
		if rel != "." {
			dir = filepath.Join(abs, rel)
		}
		if p := portFromEnvFiles(dir); p > 0 && res.BackendPort == project.DetectPublicPort(root) {
			// keep backend from capture root unless only found in web api url
		}
		if p := apiURLFromEnv(dir); p > 0 {
			res.BackendPort = p
		}
		if p := frontendPortFromEnv(dir); p > 0 {
			res.FrontendPort = p
		}
	}

	if res.BackendPort <= 0 {
		res.BackendPort = 4000
	}
	if res.FrontendPort <= 0 {
		res.FrontendPort = 3000
	}

	if IsListening("127.0.0.1", res.BackendPort) {
		res.BackendPID = PIDListeningOn("127.0.0.1", res.BackendPort)
	}
	res.InspectPort = FindInspectPort()
	return res
}

func portFromEnvFiles(dir string) int {
	for _, name := range []string{".env", ".env.local", ".env.development"} {
		if p := portFromEnvFile(filepath.Join(dir, name)); p > 0 {
			return p
		}
	}
	return 0
}

func portFromEnvFile(path string) int {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "PORT=") {
			v := strings.Trim(strings.TrimPrefix(line, "PORT="), "\"'")
			if p, err := strconv.Atoi(v); err == nil && p > 0 {
				return p
			}
		}
	}
	return 0
}

func apiURLFromEnv(dir string) int {
	for _, name := range []string{".env", ".env.local", ".env.development"} {
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			continue
		}
		if m := apiURLRe.FindStringSubmatch(string(data)); len(m) == 2 {
			if p, err := strconv.Atoi(m[1]); err == nil && p > 0 {
				return p
			}
		}
	}
	return 0
}

func frontendPortFromEnv(dir string) int {
	for _, name := range []string{".env", ".env.local"} {
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			continue
		}
		for _, line := range strings.Split(string(data), "\n") {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "NEXTAUTH_URL=") || strings.HasPrefix(line, "NEXT_PUBLIC_APP_URL=") {
				if p := portFromURL(strings.Trim(strings.SplitN(line, "=", 2)[1], "\"' ")); p > 0 {
					return p
				}
			}
		}
	}
	return 0
}

func portFromURL(raw string) int {
	raw = strings.TrimSpace(raw)
	if !strings.Contains(raw, "://") {
		return 0
	}
	hostPort := strings.TrimPrefix(strings.SplitN(raw, "://", 2)[1], "/")
	if strings.Contains(hostPort, ":") {
		parts := strings.Split(hostPort, ":")
		if p, err := strconv.Atoi(parts[len(parts)-1]); err == nil {
			return p
		}
	}
	if strings.Contains(raw, ":3000") {
		return 3000
	}
	return 0
}

// IsListening returns true when host:port accepts TCP connections.
func IsListening(host string, port int) bool {
	conn, err := net.DialTimeout("tcp", net.JoinHostPort(host, itoa(port)), 250*time.Millisecond)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

// FindInspectPort probes common Node inspector ports.
func FindInspectPort() int {
	client := &http.Client{Timeout: 300 * time.Millisecond}
	for _, port := range []int{9229, 9230, 9231, 9232} {
		resp, err := client.Get("http://127.0.0.1:" + itoa(port) + "/json/list")
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return port
			}
		}
	}
	return 0
}

func itoa(n int) string {
	return strconv.Itoa(n)
}
