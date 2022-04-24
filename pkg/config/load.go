package config

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

func Load(localRepo, filename string) (*Config, error) {
	f, err := os.ReadFile(filepath.Join(localRepo, filename))
	if err != nil {
		return nil, err
	}
	conf := &Config{}
	if err := yaml.Unmarshal(f, conf); err != nil {
		return nil, err
	}
	return conf, nil
}
