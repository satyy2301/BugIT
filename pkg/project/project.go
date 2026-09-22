package project

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	DirName      = ".bugit"
	ConfigName   = "bugit.yaml"
	ReplayName   = "replay.yaml"
	SnapshotsDir = "snapshots"
	DataDir      = "data"
	LatestLink   = "latest.dre"
)

// Layout holds standard paths under a capture root.
type Layout struct {
	Root           string
	BugitDir       string
	ConfigPath     string
	ReplayPath     string
	SnapshotsPath  string
	DataPath       string
	LatestSnapshot string
}

// Config is persisted local capture settings.
type Config struct {
	CollectorHTTP string `yaml:"collector_http"`
	CollectorGRPC string `yaml:"collector_grpc"`
	RecordProxy   string `yaml:"record_proxy"`
	AppPort       int    `yaml:"app_port"`
	SnapshotKey   string `yaml:"snapshot_key"`
	Runtime       string `yaml:"runtime,omitempty"`
	InspectPort   int    `yaml:"inspect_port,omitempty"`
	CaptureRoot   string `yaml:"capture_root,omitempty"`
	DevCommand    string `yaml:"dev_command,omitempty"`
}

// CaptureTarget describes where and how to run capture.
type CaptureTarget struct {
	Root       string
	DevCommand []string
	PublicPort int
}

func DefaultConfig() Config {
	return Config{
		CollectorHTTP: "http://127.0.0.1:28080",
		CollectorGRPC: "127.0.0.1:29090",
		RecordProxy:   "127.0.0.1:4000",
		AppPort:       4000,
		SnapshotKey:   "dev-insecure-key-change-me",
		InspectPort:   9229,
		DevCommand:    "npm run dev",
	}
}

// FindRoot returns nearest ancestor with a project marker (legacy).
func FindRoot(start string) string {
	return FindCaptureRoot(start)
}

// FindCaptureRoot picks the best directory to run npm/go dev from.
func FindCaptureRoot(workspace string) string {
	abs, err := filepath.Abs(workspace)
	if err != nil {
		return workspace
	}

	// Cached in existing bugit.yaml at workspace level
	if cached := readCachedCaptureRoot(abs); cached != "" {
		if _, err := os.Stat(filepath.Join(cached, "package.json")); err == nil {
			return cached
		}
	}

	if hasDevPackageJSON(abs) {
		return abs
	}

	// Monorepo: scan common subfolders
	for _, sub := range []string{"backend", "server", "api", "apps/api", "packages/api"} {
		candidate := filepath.Join(abs, sub)
		if hasDevPackageJSON(candidate) {
			return candidate
		}
	}

	// Walk up for package.json with dev script (prefer over .git-only roots)
	dir := abs
	for {
		if hasDevPackageJSON(dir) {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	return abs
}

func readCachedCaptureRoot(workspace string) string {
	for _, base := range []string{workspace, filepath.Dir(workspace)} {
		path := filepath.Join(base, DirName, ConfigName)
		cfg, err := LoadConfig(path)
		if err != nil || cfg.CaptureRoot == "" {
			continue
		}
		if filepath.IsAbs(cfg.CaptureRoot) {
			return cfg.CaptureRoot
		}
		return filepath.Join(base, cfg.CaptureRoot)
	}
	return ""
}

func hasDevPackageJSON(dir string) bool {
	path := filepath.Join(dir, "package.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	var pkg struct {
		Scripts map[string]string `json:"scripts"`
	}
	if err := json.Unmarshal(data, &pkg); err != nil {
		return false
	}
	if pkg.Scripts["dev"] != "" || pkg.Scripts["start"] != "" {
		return true
	}
	return false
}

// ResolveCaptureTarget finds root, dev command, and public port.
func ResolveCaptureTarget(workspace string) CaptureTarget {
	root := FindCaptureRoot(workspace)
	cfgPath := filepath.Join(root, DirName, ConfigName)
	cfg, _ := LoadConfig(cfgPath)

	detected := DetectPublicPort(root)
	port := cfg.AppPort
	if port <= 0 || port != detected {
		port = detected
	}

	cmd := cfg.DevCommand
	if cmd == "" {
		cmd = DetectDevCommand(root)
	}

	return CaptureTarget{
		Root:       root,
		DevCommand: ParseDevCommand(cmd),
		PublicPort: port,
	}
}

func DetectDevCommand(root string) string {
	path := filepath.Join(root, "package.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return "npm run dev"
	}
	var pkg struct {
		Scripts map[string]string `json:"scripts"`
	}
	if err := json.Unmarshal(data, &pkg); err != nil {
		return "npm run dev"
	}
	if pkg.Scripts["dev"] != "" {
		return "npm run dev"
	}
	if pkg.Scripts["start"] != "" {
		return "npm start"
	}
	return "npm run dev"
}

func ParseDevCommand(cmd string) []string {
	parts := strings.Fields(strings.TrimSpace(cmd))
	if len(parts) == 0 {
		return []string{"npm", "run", "dev"}
	}
	return parts
}

func DetectPublicPort(root string) int {
	for _, name := range []string{".env", ".env.local", ".env.development"} {
		if p := portFromEnvFile(filepath.Join(root, name)); p > 0 {
			return p
		}
	}
	if hasDevPackageJSON(root) {
		return 4000
	}
	return 3000
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

// PrepareConfig updates cfg for capture: public port on proxy, internal offset for app.
func PrepareConfig(root string, cfg Config) Config {
	if cfg.AppPort <= 0 {
		cfg.AppPort = DetectPublicPort(root)
	}
	cfg.RecordProxy = fmtHostPort("127.0.0.1", cfg.AppPort)
	if cfg.DevCommand == "" {
		cfg.DevCommand = DetectDevCommand(root)
	}
	cfg.CaptureRoot = root
	return cfg
}

func fmtHostPort(host string, port int) string {
	return host + ":" + strconv.Itoa(port)
}

func InternalPort(publicPort int) int {
	return publicPort + 10000
}

func LayoutFor(root string) Layout {
	bugit := filepath.Join(root, DirName)
	return Layout{
		Root:           root,
		BugitDir:       bugit,
		ConfigPath:     filepath.Join(bugit, ConfigName),
		ReplayPath:     filepath.Join(bugit, ReplayName),
		SnapshotsPath:  filepath.Join(bugit, SnapshotsDir),
		DataPath:       filepath.Join(bugit, DataDir),
		LatestSnapshot: filepath.Join(bugit, LatestLink),
	}
}

func EnsureLayout(root string) (Layout, error) {
	layout := LayoutFor(root)
	for _, dir := range []string{layout.BugitDir, layout.SnapshotsPath, layout.DataPath} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return layout, err
		}
	}
	if err := ensureGitignore(root); err != nil {
		return layout, err
	}
	return layout, nil
}

func LoadConfig(path string) (Config, error) {
	cfg := DefaultConfig()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return cfg, err
	}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return cfg, err
	}
	if cfg.SnapshotKey == "" {
		cfg.SnapshotKey = DefaultConfig().SnapshotKey
	}
	return cfg, nil
}

func SaveConfig(path string, cfg Config) error {
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

// SyncWorkspaceConfig writes capture settings from workspace detection into bugit.yaml.
func SyncWorkspaceConfig(workspace string) (Layout, Config, error) {
	root := FindCaptureRoot(workspace)
	layout, err := EnsureLayout(root)
	if err != nil {
		return layout, Config{}, err
	}
	cfg, err := LoadConfig(layout.ConfigPath)
	if err != nil {
		return layout, Config{}, err
	}
	cfg = PrepareConfig(root, cfg)
	if err := SaveConfig(layout.ConfigPath, cfg); err != nil {
		return layout, cfg, err
	}
	return layout, cfg, nil
}

func ensureGitignore(root string) error {
	path := filepath.Join(root, ".gitignore")
	data, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	content := string(data)
	if strings.Contains(content, DirName+"/") || strings.Contains(content, DirName+"\\") {
		return nil
	}
	line := DirName + "/\n"
	if len(data) > 0 && !strings.HasSuffix(content, "\n") {
		line = "\n" + line
	}
	return os.WriteFile(path, append(data, []byte(line)...), 0o644)
}
