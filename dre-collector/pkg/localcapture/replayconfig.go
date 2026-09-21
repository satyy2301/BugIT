package localcapture

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/bugit/dre-engine/api/ioevent"
	"gopkg.in/yaml.v3"
)

type replayYAML struct {
	ProxyAddr  string           `yaml:"proxy_addr"`
	DebugAddr  string           `yaml:"debug_addr"`
	TimeFreeze replayTimeFreeze `yaml:"time_freeze"`
	Services   []replayService  `yaml:"services"`
}

type replayTimeFreeze struct {
	Enabled  bool   `yaml:"enabled"`
	ShimPath string `yaml:"shim_path"`
}

type replayService struct {
	Name       string `yaml:"name"`
	LocalPort  int    `yaml:"local_port"`
	RemoteHost string `yaml:"remote_host"`
	Protocol   string `yaml:"protocol"`
}

// WriteReplayConfig generates .bugit/replay.yaml from captured events.
func WriteReplayConfig(path, projectRoot string, events []ioevent.IOEvent) error {
	cfg := replayYAML{
		ProxyAddr: "127.0.0.1:18080",
		DebugAddr: "127.0.0.1:19090",
		TimeFreeze: replayTimeFreeze{
			Enabled: false,
		},
	}

	type svcKey struct {
		host string
		port int
	}
	seen := map[svcKey]bool{}
	localPort := 18081

	for _, ev := range events {
		payload := string(ev.Payload[:ev.PayloadLen])
		host := parseHTTPHost(payload)
		if host == "" {
			continue
		}
		h, p := splitServiceHost(host)
		key := svcKey{host: h, port: p}
		if seen[key] {
			continue
		}
		seen[key] = true
		name := sanitizeServiceName(h)
		cfg.Services = append(cfg.Services, replayService{
			Name:       name,
			LocalPort:  localPort,
			RemoteHost: fmtHostPort(h, p),
			Protocol:   "http",
		})
		localPort++
	}

	sort.Slice(cfg.Services, func(i, j int) bool {
		return cfg.Services[i].Name < cfg.Services[j].Name
	})

	if shim := findClockShim(projectRoot); shim != "" {
		cfg.TimeFreeze.Enabled = true
		cfg.TimeFreeze.ShimPath = shim
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func splitServiceHost(host string) (string, int) {
	if strings.Contains(host, ":") {
		parts := strings.Split(host, ":")
		port := 80
		fmt.Sscanf(parts[len(parts)-1], "%d", &port)
		return strings.Join(parts[:len(parts)-1], ":"), port
	}
	return host, 80
}

func fmtHostPort(host string, port int) string {
	if port == 80 {
		return host
	}
	return fmt.Sprintf("%s:%d", host, port)
}

func sanitizeServiceName(host string) string {
	host = strings.ReplaceAll(host, ".", "-")
	host = strings.ReplaceAll(host, ":", "-")
	if host == "" {
		return "service"
	}
	return host
}

func findClockShim(root string) string {
	for _, rel := range []string{"bin/clock_shim.so", ".bugit/clock_shim.so"} {
		p := filepath.Join(root, rel)
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}
