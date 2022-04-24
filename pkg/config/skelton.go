package config

import (
	_ "embed"

	"gopkg.in/yaml.v3"
)

//go:embed skelton.yaml
var skelton []byte

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
	return skelton, nil
}
