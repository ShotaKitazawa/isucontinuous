package mcpserver

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/ShotaKitazawa/isucontinuous/pkg/cmd"
)

// Runners are the entry points the tools delegate to; tests replace them.
type Runners struct {
	Setup      func(context.Context, cmd.ConfigSetup) (*cmd.SetupResult, error)
	Deploy     func(context.Context, cmd.ConfigDeploy) (*cmd.DeployResult, error)
	Profiling  func(context.Context, cmd.ConfigProfiling) (*cmd.ProfilingResult, error)
	AfterBench func(context.Context, cmd.ConfigAfterBench) (*cmd.AfterBenchResult, error)
	Release    func(context.Context, cmd.ConfigRelease) (*cmd.ReleaseResult, error)
	Status     func(context.Context, cmd.ConfigStatus) (*cmd.StatusResult, error)
	Diff       func(context.Context, cmd.ConfigDiff) (*cmd.DiffResult, error)
}

func DefaultRunners() Runners {
	return Runners{
		Setup:      cmd.RunSetupContext,
		Deploy:     cmd.RunDeployContext,
		Profiling:  cmd.RunProfilingContext,
		AfterBench: cmd.RunAfterBenchContext,
		Release:    cmd.RunReleaseContext,
		Status:     cmd.RunStatusContext,
		Diff:       cmd.RunDiffContext,
	}
}

type Options struct {
	// Common leaves Owner empty so that locks are held as user@host, the same as the CLI.
	Common     cmd.ConfigCommon
	SlackToken string
	// Limits bound the artifacts in the afterbench response; the zero value means the defaults.
	Limits Limits
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
