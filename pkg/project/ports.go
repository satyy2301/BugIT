package project

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

var apiURLRe = regexp.MustCompile(`(?i)(?:NEXT_PUBLIC_API_URL|API_URL|VITE_API_URL)\s*=\s*https?://[^:]+:(\d+)`)

// ResolveBackendPort picks the backend API port for attach/capture.
func ResolveBackendPort(workspace, captureRoot string) int {
	cfgPath := filepath.Join(captureRoot, DirName, ConfigName)
	if _, err := os.Stat(cfgPath); err == nil {
		if cfg, err := LoadConfig(cfgPath); err == nil && cfg.AppPort > 0 {
			return cfg.AppPort
		}
	}

	if p := portFromEnvDirs(captureRoot); p > 0 {
		return p
	}

	abs, err := filepath.Abs(workspace)
	if err != nil {
		abs = workspace
	}
	for _, rel := range []string{"web", "frontend", "client", "apps/web"} {
		dir := filepath.Join(abs, rel)
		if p := apiURLFromEnv(dir); p > 0 {
			return p
		}
	}
	if p := apiURLFromEnv(abs); p > 0 {
		return p
	}

	if hasDevPackageJSON(captureRoot) {
		return 4000
	}
	return 3000
}

func portFromEnvDirs(dir string) int {
	for _, name := range []string{".env", ".env.local", ".env.development"} {
		if p := portFromEnvFile(filepath.Join(dir, name)); p > 0 {
			return p
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
