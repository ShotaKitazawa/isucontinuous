package config

type Config struct {
	Setup Setup  `yaml:"setup,omitempty"`
	Slack Slack  `yaml:"slack,omitempty"`
	Hosts []Host `yaml:"hosts,omitempty"`

	// Disabled holds the hosts that Load dropped, so that commands which clean up (release, status) can still reach them.
	Disabled []DisabledHost `yaml:"-"`

	// Warnings is filled by Load and never read from yaml.
	Warnings []string `yaml:"-"`
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
	DefaultChannelId string `yaml:"default_channel_id,omitempty"`
}

type Host struct {
	Name       string     `yaml:"name,omitempty"`
	Host       string     `yaml:"host,omitempty"`
	Local      bool       `yaml:"local,omitempty"`
	Disabled   bool       `yaml:"disabled,omitempty"`
	PortExpr   string     `yaml:"port,omitempty"`
	Port       int        `yaml:"-"`
	User       string     `yaml:"user,omitempty"`
	Key        string     `yaml:"key,omitempty"`
	Password   string     `yaml:"password,omitempty"`
	Deploy     Deploy     `yaml:"deploy,omitempty"`
	Profiling  Profiling  `yaml:"profiling,omitempty"`
	AfterBench AfterBench `yaml:"after_bench,omitempty"`
}

func (c Host) ListTarget() []DeployTarget {
	return c.Deploy.Targets
}

type Deploy struct {
	SlackChannelId string         `yaml:"slack_channel_id,omitempty"`
	PreCommand     string         `yaml:"pre_command,omitempty"`
	PostCommand    string         `yaml:"post_command,omitempty"`
	Targets        []DeployTarget `yaml:"targets,omitempty"`
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
	SlackChannelId string `yaml:"slack_channel_id,omitempty"`
	Target         string `yaml:"target,omitempty"`
	Command        string `yaml:"command,omitempty"`
}

// DisabledHost is a host with `disabled: true`. Host is usable only if Err is nil.
type DisabledHost struct {
	Name string
	Host Host
	Err  error
}
