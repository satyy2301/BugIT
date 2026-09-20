package trigger

import (
	"os"

	"gopkg.in/yaml.v3"
)

type RulesConfig struct {
	HTTP5xx []string `yaml:"http_5xx"`
}

func LoadRules(path string) ([]string, error) {
	if path == "" {
		return nil, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg RulesConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return cfg.HTTP5xx, nil
}

func (e *Engine) SetRules5xx(rules []string) {
	if len(rules) == 0 {
		return
	}
	e.rules5xx = rules
}
