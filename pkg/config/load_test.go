package config

import (
	"strings"
	"testing"
)

func TestLoad(t *testing.T) {
	t.Setenv("LOAD_TEST_HOST", "192.0.2.10")
	t.Setenv("LOAD_TEST_PORT", "2222")
	cases := []struct {
		name    string
		yaml    string
		wantErr string
	}{
		{"local host without local flag", "hosts:\n- host: localhost\n", `"local: true"`},
		{"loopback host without local flag", "hosts:\n- host: 127.0.0.1\n", `"local: true"`},
		{"all disabled", "hosts:\n- host: 192.0.2.1\n  disabled: true\n", "all hosts are disabled"},
		{"duplicate name", "hosts:\n- host: 192.0.2.1\n  name: a\n- host: 192.0.2.2\n  name: a\n", "already used"},
		{"no host nor name", "hosts:\n- user: root\n", "required"},
		{"git revision in host", "hosts:\n- host: '{{.Git.Revision}}'\n", "host:"},
		{"missing env in host", "hosts:\n- host: '{{.Env.LOAD_TEST_NOT_SET}}'\n", "host:"},
		{"invalid port", "hosts:\n- host: 192.0.2.1\n  port: abc\n", "port:"},
		{"missing env in command", "hosts:\n- host: 192.0.2.1\n  deploy:\n    pre_command: '{{.Env.LOAD_TEST_NOT_SET}}'\n", "deploy.pre_command"},
		{"missing env in compile", "hosts:\n- host: 192.0.2.1\n  deploy:\n    targets:\n    - src: go\n      compile: '{{.Env.LOAD_TEST_NOT_SET}}'\n", "compile"},
		{"missing env in after_bench target", "hosts:\n- host: 192.0.2.1\n  after_bench:\n    target: '{{.Env.LOAD_TEST_NOT_SET}}'\n", "after_bench.target"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := Load([]byte(c.yaml))
			if err == nil || !strings.Contains(err.Error(), c.wantErr) {
				t.Errorf("err = %v, want containing %q", err, c.wantErr)
			}
		})
	}

	t.Run("expands connection settings and defaults name", func(t *testing.T) {
		conf, err := Load([]byte("hosts:\n- host: '{{.Env.LOAD_TEST_HOST}}'\n  port: '{{.Env.LOAD_TEST_PORT}}'\n  user: '{{.Env.LOAD_TEST_HOST}}'\n"))
		if err != nil {
			t.Fatal(err)
		}
		h := conf.Hosts[0]
		if h.Host != "192.0.2.10" || h.Name != "192.0.2.10" || h.Port != 2222 || h.User != "192.0.2.10" {
			t.Errorf("host = %+v", h)
		}
	})

	t.Run("numeric port", func(t *testing.T) {
		conf, err := Load([]byte("hosts:\n- host: 192.0.2.1\n  port: 2222\n"))
		if err != nil {
			t.Fatal(err)
		}
		if conf.Hosts[0].Port != 2222 {
			t.Errorf("Port = %d, want 2222", conf.Hosts[0].Port)
		}
	})

	t.Run("name is kept apart from host", func(t *testing.T) {
		conf, err := Load([]byte("hosts:\n- host: 192.0.2.1\n  name: isu1\n"))
		if err != nil {
			t.Fatal(err)
		}
		if h := conf.Hosts[0]; h.Name != "isu1" || h.Host != "192.0.2.1" {
			t.Errorf("host = %+v", h)
		}
	})

	t.Run("local host with local flag", func(t *testing.T) {
		conf, err := Load([]byte("hosts:\n- host: localhost\n  local: true\n"))
		if err != nil {
			t.Fatal(err)
		}
		if h := conf.Hosts[0]; !h.Local || h.Name != "localhost" {
			t.Errorf("host = %+v", h)
		}
	})

	t.Run("disabled hosts are dropped without being validated", func(t *testing.T) {
		conf, err := Load([]byte("hosts:\n" +
			"- host: '{{.Env.LOAD_TEST_NOT_SET}}'\n  disabled: true\n" +
			"- host: localhost\n  disabled: true\n" +
			"- host: 192.0.2.1\n"))
		if err != nil {
			t.Fatal(err)
		}
		if len(conf.Hosts) != 1 || conf.Hosts[0].Host != "192.0.2.1" {
			t.Errorf("hosts = %+v", conf.Hosts)
		}
	})

	t.Run("git revision is allowed in commands", func(t *testing.T) {
		_, err := Load([]byte("hosts:\n- host: 192.0.2.1\n  profiling:\n    command: 'echo {{.Git.Revision}}'\n  after_bench:\n    target: /p/{{.Git.Revision}}\n"))
		if err != nil {
			t.Fatal(err)
		}
	})

	t.Run("no hosts", func(t *testing.T) {
		if _, err := Load([]byte("slack:\n  default_channel_id: C1\n")); err != nil {
			t.Fatal(err)
		}
	})
}

func TestLoad_Skelton(t *testing.T) {
	t.Setenv("ACCESSLOG_PATH", "/var/log/nginx/access.log")
	t.Setenv("SLOWLOG_PATH", "/var/log/mysql/slow.log")
	data, err := SkeltonBytes()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Load(data); err != nil {
		t.Fatal(err)
	}
}

func TestHasDisabledHost(t *testing.T) {
	for yaml, want := range map[string]bool{
		"hosts:\n- host: '{{.Env.LOAD_TEST_NOT_SET}}'\n  disabled: true\n": true,
		"hosts:\n- host: 192.0.2.1\n":                                      false,
	} {
		got, err := HasDisabledHost([]byte(yaml))
		if err != nil || got != want {
			t.Errorf("HasDisabledHost(%q) = %v, %v; want %v", yaml, got, err, want)
		}
	}
}
