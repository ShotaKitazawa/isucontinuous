package config

type Config struct {
	Setup Setup  `yaml:"setup,omitempty"`
	Slack Slack  `yaml:"slack,omitempty"`
	Hosts []Host `yaml:"targets,omitempty"`
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
		version = c.Setup.Alp.Version
	}
	return true, version
}

func (c Config) ListTargetHosts() []Host {
	return c.Hosts
}

type Setup struct {
	Docker *Docker `yaml:"docker,omitempty"`
	Alp    *Alp    `yaml:"alp,omitempty"`
}

type Docker struct {
	Netdata *Netdata `yaml:"netdata,omitempty"`
}

type Netdata struct {
	Version    string `yaml:"version,omitempty"`
	PublicPort int    `yaml:"public_port,omitempty"`
}

type Alp struct {
	Version string `yaml:"version,omitempty"`
}

type Slack struct {
	DefaultChannel string `yaml:"default_channel,omitempty"`
}

type Host struct {
	Host       string     `yaml:"host,omitempty"`
	Port       int        `yaml:"int,omitempty"`
	User       string     `yaml:"user,omitempty"`
	Key        string     `yaml:"key,omitempty"`
	Password   string     `yaml:"password,omitempty"`
	Deploy     Deploy     `yaml:"deploy,omitempty"`
	Profiling  Profiling  `yaml:"profiling,omitempty"`
	AfterBench AfterBench `yaml:"after_bench,omitempty"`
}

func (c Host) IsLocal() bool {
	return c.Host == "localhost" || c.Host == "127.0.0.1"
}

func (c Host) ListTarget() []DeployTarget {
	return c.Deploy.Targets
}

type Deploy struct {
	SlackChannel string         `yaml:"slack_channel,omitempty"`
	PreCommand   string         `yaml:"pre_command,omitempty"`
	PostCommand  string         `yaml:"post_command,omitempty"`
	Targets      []DeployTarget `yaml:"targets,omitempty"`
}

type DeployTarget struct {
	Src     string `yaml:"src,omitempty"`
	Target  string `yaml:"target,omitempty"`
	Compile string `yaml:"compile,omitempty"`
}

type Profiling struct {
	Command string `yaml:"command,omitempty"`
}

type AfterBench struct {
	SlackChannel string `yaml:"slack_channel,omitempty"`
	Target       string `yaml:"target,omitempty"`
	Command      string `yaml:"command,omitempty"`
}
