package config

import (
	"os"

	"gopkg.in/yaml.v3"
)

func Load(filename string) (*Config, error) {
	f, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	conf := &Config{}
	if err := yaml.Unmarshal(f, conf); err != nil {
		return nil, err
	}
	return conf, nil
}

func (c Config) IsDockerEnabled() bool {
	return c.Setup.Docker != nil
}

func (c Config) IsNetdataEnabled() (flag bool, version string, port int) {
	if !(c.Setup.Docker != nil && c.Setup.Docker.Netdata != nil) {
		return false, "", 0
	}
	if c.Setup.Docker.Netdata.Version == "" {
		version = "latest"
	} else {
		version = c.Setup.Docker.Netdata.Version
	}
	if c.Setup.Docker.Netdata.PublicPort == 0 {
		port = 19999
	} else {
		port = c.Setup.Docker.Netdata.PublicPort
	}
	return true, version, port
}

func (c Config) IsAlpEnabled() (flag bool, version string) {
	if !(c.Setup.Alp != nil) {
		return false, ""
	}
	if c.Setup.Alp.Version == "" {
		version = "latest"
	} else {
		version = c.Setup.Docker.Netdata.Version
	}
	return true, version
}
