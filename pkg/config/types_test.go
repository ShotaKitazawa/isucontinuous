package config

import (
	"testing"

	"gopkg.in/yaml.v3"
)

func TestHost_Port(t *testing.T) {
	h := Host{}
	if err := yaml.Unmarshal([]byte("host: 192.0.2.1\nport: 2222\n"), &h); err != nil {
		t.Fatal(err)
	}
	if h.Port != 2222 {
		t.Errorf("Host.Port = %d, want 2222", h.Port)
	}
}
