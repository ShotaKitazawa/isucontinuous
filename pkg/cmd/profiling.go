package cmd

import (
	"context"
	"fmt"

	"go.uber.org/zap"
	"k8s.io/utils/exec"

	"github.com/ShotaKitazawa/isucontinuous/pkg/config"
	"github.com/ShotaKitazawa/isucontinuous/pkg/localrepo"
	"github.com/ShotaKitazawa/isucontinuous/pkg/shell"
	"github.com/ShotaKitazawa/isucontinuous/pkg/template"
	"github.com/ShotaKitazawa/isucontinuous/pkg/usecases/profiling"
)

type ConfigProfiling struct {
	ConfigCommon
}

func RunProfiling(conf ConfigProfiling) (*ProfilingResult, error) {
	ctx := context.Background()
	logger, err := newLogger(conf.LogLevel, conf.LogFilename)
	if err != nil {
		return nil, err
	}
	shell.SetKnownHostsPath(conf.KnownHostsPath)
	// Attach local-repo
	repo, err := localrepo.AttachLocalRepo(logger, exec.New(), conf.LocalRepoPath)
	if err != nil {
		return nil, err
	}
	// Set newProfilingersFunc

	return runProfiling(conf, ctx, logger, repo, profiling.New)
}

func runProfiling(
	conf ConfigProfiling, ctx context.Context, logger *zap.Logger,
	repo localrepo.LocalRepoIface, newProfilingersFunc profiling.NewFunc,
) (*ProfilingResult, error) {
	logger.Info("start profiling")
	defer func() { logger.Info("finish profiling") }()
	// Load isucontinus.yaml
	isucontinuous, err := repo.LoadConf()
	if err != nil {
		return nil, err
	}
	// Get revision
	gitRevision, err := repo.GetRevision(ctx)
	if err != nil {
		return nil, fmt.Errorf("%s/.revision is not found. exec `deploy` command first", conf.LocalRepoPath)
	}
	// Profiling files to per host
	hosts, err := perHostExec(logger, ctx, conf.reporter(), isucontinuous.Hosts, []task{{
		"Profiling",
		func(ctx context.Context, host config.Host) error {
			profilinger, err := newProfilingersFunc(logger, template.New(gitRevision), host)
			if err != nil {
				return err
			}
			if err := profilinger.Profiling(ctx, host.Profiling.Command); err != nil {
				return err
			}
			return nil
		},
	}})
	return &ProfilingResult{Result{Hosts: hosts, Warnings: isucontinuous.Warnings}}, err
}
