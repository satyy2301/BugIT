package config

import (
	"os"

	"gopkg.in/yaml.v3"
)

type ReplayConfig struct {
	ProxyAddr string        `yaml:"proxy_addr"`
	DebugAddr string        `yaml:"debug_addr"`
	Services  []ServiceRule `yaml:"services"`
	TimeFreeze TimeFreeze   `yaml:"time_freeze"`
}

type ServiceRule struct {
	Name       string `yaml:"name"`
	LocalPort  int    `yaml:"local_port"`
	RemoteHost string `yaml:"remote_host"`
	Protocol   string `yaml:"protocol"` // http or grpc
}

type TimeFreeze struct {
	Enabled  bool   `yaml:"enabled"`
	ShimPath string `yaml:"shim_path"`
}

func Load(path string) (ReplayConfig, error) {
	cfg := ReplayConfig{
		ProxyAddr: "127.0.0.1:18080",
		DebugAddr: "127.0.0.1:19090",
		TimeFreeze: TimeFreeze{Enabled: false, ShimPath: "bin/clock_shim.so"},
	}
	if path == "" {
		return cfg, nil
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return cfg, err
	}
	if err := yaml.Unmarshal(b, &cfg); err != nil {
		return cfg, err
	}
	return cfg, nil
}
