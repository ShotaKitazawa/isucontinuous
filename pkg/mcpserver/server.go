package mcpserver

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/ShotaKitazawa/isucontinuous/pkg/cmd"
)

// Runners are the entry points the tools delegate to; tests replace them.
type Runners struct {
	Init       func(cmd.ConfigInit) (*cmd.InitResult, error)
	Setup      func(cmd.ConfigSetup) (*cmd.SetupResult, error)
	Import     func(cmd.ConfigImport) (*cmd.ImportResult, error)
	Push       func(cmd.ConfigPush) (*cmd.PushResult, error)
	Sync       func(cmd.ConfigSync) (*cmd.SyncResult, error)
	Deploy     func(cmd.ConfigDeploy) (*cmd.DeployResult, error)
	Profiling  func(cmd.ConfigProfiling) (*cmd.ProfilingResult, error)
	AfterBench func(cmd.ConfigAfterBench) (*cmd.AfterBenchResult, error)
	Release    func(cmd.ConfigRelease) (*cmd.ReleaseResult, error)
	Status     func(cmd.ConfigStatus) (*cmd.StatusResult, error)
	Diff       func(cmd.ConfigDiff) (*cmd.DiffResult, error)
}

func DefaultRunners() Runners {
	return Runners{
		Init:       cmd.RunInit,
		Setup:      cmd.RunSetup,
		Import:     cmd.RunImport,
		Push:       cmd.RunPush,
		Sync:       cmd.RunSync,
		Deploy:     cmd.RunDeploy,
		Profiling:  cmd.RunProfiling,
		AfterBench: cmd.RunAfterBench,
		Release:    cmd.RunRelease,
		Status:     cmd.RunStatus,
		Diff:       cmd.RunDiff,
	}
}

type Options struct {
	// Common leaves Owner empty so that locks are held as user@host, the same as the CLI.
	Common     cmd.ConfigCommon
	SlackToken string
	// Runners nil means the real cmd.Run* functions.
	Runners *Runners
	Version string
}

func New(opts Options) *mcp.Server {
	if opts.Runners == nil {
		r := DefaultRunners()
		opts.Runners = &r
	}
	server := mcp.NewServer(&mcp.Implementation{Name: "isucontinuous", Version: opts.Version}, nil)
	registerTools(server, opts)
	return server
}

// Run serves on stdio; nothing else may write to stdout.
func Run(ctx context.Context, opts Options) error {
	return New(opts).Run(ctx, &mcp.StdioTransport{})
}
