package config

type Config struct {
	Setup Setup  `json:"setup,omitempty"`
	Slack Slack  `json:"slack,omitempty"`
	Hosts []Host `json:"targets,omitempty"`
}

type Setup struct {
	Docker *Docker `json:"docker,omitempty"`
	Alp    *Alp    `json:"alp,omitempty"`
}

type Docker struct {
	Netdata *Netdata `json:"netdata,omitempty"`
}

type Netdata struct {
	Version    string `json:"version,omitempty"`
	PublicPort int    `json:"public_port,omitempty"`
}

type Alp struct {
	Version string `json:"version,omitempty"`
}

type Slack struct {
	DefaultChannel string `json:"default_channel,omitempty"`
	Token          string `json:"token,omitempty"`
}

type Host struct {
	Host       string     `json:"host,omitempty"`
	User       string     `json:"user,omitempty"`
	Key        string     `json:"key,omitempty"`
	Password   string     `json:"password,omitempty"`
	Deploy     Deploy     `json:"deploy,omitempty"`
	Profiling  Profiling  `json:"profiling,omitempty"`
	AfterBench AfterBench `json:"after_bench,omitempty"`
}

type Deploy struct {
	SlackChannel string         `json:"slack_channel,omitempty"`
	PreCommand   string         `json:"pre_command,omitempty"`
	PostCommand  string         `json:"post_command,omitempty"`
	Targets      []DeployTarget `json:"targets,omitempty"`
}

type DeployTarget struct {
	Src     string `json:"src,omitempty"`
	Target  string `json:"target,omitempty"`
	Compile string `json:"compile,omitempty"`
}

type Profiling struct {
	Command string `json:"command,omitempty"`
}

type AfterBench struct {
	SlackChannel string `json:"slack_channel,omitempty"`
	Target       string `json:"target,omitempty"`
	Command      string `json:"command,omitempty"`
}
