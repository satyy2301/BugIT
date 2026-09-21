package project

import (
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	DirName       = ".bugit"
	ConfigName    = "bugit.yaml"
	ReplayName    = "replay.yaml"
	SnapshotsDir  = "snapshots"
	DataDir       = "data"
	LatestLink    = "latest.dre"
)

// Layout holds standard paths under a project root.
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
}

func DefaultConfig() Config {
	return Config{
		CollectorHTTP: "http://127.0.0.1:28080",
		CollectorGRPC: "127.0.0.1:29090",
		RecordProxy:   "127.0.0.1:28081",
		AppPort:       3000,
		SnapshotKey:   "dev-insecure-key-change-me",
		InspectPort:   9229,
	}
}

func FindRoot(start string) string {
	dir, err := filepath.Abs(start)
	if err != nil {
		return start
	}
	for {
		if hasProjectMarker(dir) {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return start
}

func hasProjectMarker(dir string) bool {
	for _, name := range []string{"package.json", "go.mod", "pyproject.toml", "Cargo.toml", ".git"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err == nil {
			return true
		}
	}
	return false
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
	return os.WriteFile(path, data, 0o644)
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
