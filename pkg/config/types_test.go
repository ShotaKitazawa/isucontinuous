package config

import "testing"

func TestHost_Port(t *testing.T) {
	conf, err := Load([]byte("hosts:\n- host: 192.0.2.1\n  port: 2222\n"))
	if err != nil {
		t.Fatal(err)
	}
	if h := conf.Hosts[0]; h.Port != 2222 {
		t.Errorf("Host.Port = %d, want 2222", h.Port)
	}
}
