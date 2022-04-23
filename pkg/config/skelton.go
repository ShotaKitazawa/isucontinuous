package config

import (
	"os"

	"gopkg.in/yaml.v3"
)

func Skelton() (Config, error) {
	f, err := SkeltonBytes()
	if err != nil {
		return Config{}, err
	}
	var conf Config
	if err := yaml.Unmarshal(f, &conf); err != nil {
		return Config{}, err
	}
	return conf, nil
}

func SkeltonBytes() ([]byte, error) {
	f, err := os.ReadFile("./skelton.yaml")
	if err != nil {
		return nil, err
	}
	return f, nil
}
