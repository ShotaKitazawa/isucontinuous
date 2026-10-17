package config

import (
	"fmt"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/ShotaKitazawa/isucontinuous/pkg/template"
)

// validationRevision is a dummy value to check templates that reference .Git.Revision.
const validationRevision = "validation"

// Load parses isucontinuous.yaml, drops disabled hosts, expands the connection
// settings and validates every template before any connection is made.
func Load(data []byte) (*Config, error) {
	conf := &Config{}
	if err := yaml.Unmarshal(data, conf); err != nil {
		return nil, err
	}
	templator := template.New(validationRevision)
	var hosts []Host
	names := make(map[string]int)
	for idx, host := range conf.Hosts {
		if host.Disabled {
			conf.Disabled = append(conf.Disabled, resolveDisabled(templator, host))
			continue
		}
		rawPassword := host.Password
		host, err := resolveHost(templator, host)
		if err != nil {
			return nil, fmt.Errorf("hosts[%d]: %w", idx, err)
		}
		if prev, ok := names[host.Name]; ok {
			return nil, fmt.Errorf("hosts[%d]: name %q is already used by hosts[%d]", idx, host.Name, prev)
		}
		names[host.Name] = idx
		if err := validateTemplates(templator, host); err != nil {
			return nil, fmt.Errorf("hosts[%d] (%s): %w", idx, host.Name, err)
		}
		if isPlaintextPassword(rawPassword) {
			conf.Warnings = append(conf.Warnings, fmt.Sprintf(
				"host %s uses password authentication: use a key instead (password in isucontinuous.yaml is discouraged)", host.Name))
		}
		hosts = append(hosts, host)
	}
	if len(conf.Hosts) > 0 && len(hosts) == 0 {
		return nil, fmt.Errorf("all hosts are disabled")
	}
	conf.Hosts = hosts
	return conf, nil
}

// resolveDisabled does not fail when the environment variables of a disabled host are unset.
func resolveDisabled(t *template.Templator, host Host) DisabledHost {
	resolved, err := resolveHost(t, host)
	if err != nil {
		name := host.Name
		if name == "" {
			name = host.Host
		}
		return DisabledHost{Name: name, Host: host, Err: err}
	}
	return DisabledHost{Name: resolved.Name, Host: resolved}
}

// HasDisabledHost reads only the disabled flags so that it works without the environment variables of the hosts.
func HasDisabledHost(data []byte) (bool, error) {
	conf := &Config{}
	if err := yaml.Unmarshal(data, conf); err != nil {
		return false, err
	}
	for _, host := range conf.Hosts {
		if host.Disabled {
			return true, nil
		}
	}
	return false, nil
}

// HasPlaintextPassword reads the raw password values so that it works without the environment variables of the hosts.
func HasPlaintextPassword(data []byte) (bool, error) {
	conf := &Config{}
	if err := yaml.Unmarshal(data, conf); err != nil {
		return false, err
	}
	for _, host := range conf.Hosts {
		if isPlaintextPassword(host.Password) {
			return true, nil
		}
	}
	return false, nil
}

func isPlaintextPassword(raw string) bool {
	return raw != "" && !strings.Contains(raw, "{{")
}

func resolveHost(t *template.Templator, host Host) (Host, error) {
	for _, f := range []struct {
		field string
		value *string
	}{
		{"host", &host.Host}, {"user", &host.User}, {"key", &host.Key},
		{"port", &host.PortExpr}, {"password", &host.Password},
	} {
		v, err := t.ExecEnv(*f.value)
		if err != nil {
			return host, fmt.Errorf("%s: %w", f.field, err)
		}
		*f.value = v
	}
	if host.PortExpr != "" {
		port, err := strconv.Atoi(host.PortExpr)
		if err != nil || port < 1 || port > 65535 {
			return host, fmt.Errorf("port: %q is not a valid port number", host.PortExpr)
		}
		host.Port = port
	}
	if !host.Local && (host.Host == "localhost" || host.Host == "127.0.0.1") {
		return host, fmt.Errorf(
			`host %q is no longer treated as local execution: set "local: true" to run on this machine, or specify the real address`,
			host.Host)
	}
	if host.Name == "" {
		host.Name = host.Host
	}
	if host.Name == "" {
		return host, fmt.Errorf(`"host" or "name" is required`)
	}
	return host, nil
}

func validateTemplates(t *template.Templator, host Host) error {
	texts := []struct{ field, text string }{
		{"deploy.pre_command", host.Deploy.PreCommand},
		{"deploy.post_command", host.Deploy.PostCommand},
		{"profiling.command", host.Profiling.Command},
		{"after_bench.target", host.AfterBench.Target},
		{"after_bench.command", host.AfterBench.Command},
	}
	for idx, target := range host.Deploy.Targets {
		texts = append(texts, struct{ field, text string }{
			fmt.Sprintf("deploy.targets[%d].compile", idx), target.Compile})
	}
	for _, f := range texts {
		if _, err := t.Exec(f.text); err != nil {
			return fmt.Errorf("%s: %w", f.field, err)
		}
	}
	return nil
}
