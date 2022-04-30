package cmd

import (
	"context"
	"fmt"

	"go.uber.org/zap"
	"k8s.io/utils/exec"

	"github.com/ShotaKitazawa/isu-continuous/pkg/config"
	"github.com/ShotaKitazawa/isu-continuous/pkg/localrepo"
	"github.com/ShotaKitazawa/isu-continuous/pkg/template"
	"github.com/ShotaKitazawa/isu-continuous/pkg/usecases/profiling"
)

type ConfigProfiling struct {
	ConfigCommon
}

func RunProfiling(conf ConfigProfiling) error {
	ctx := context.Background()
	logger, err := newLogger(conf.LogLevel, conf.LogFilename)
	if err != nil {
		return err
	}
	// Attach local-repo
	repo, err := localrepo.AttachLocalRepo(logger, exec.New(), conf.LocalRepoPath)
	if err != nil {
		return err
	}
	// Set newProfilingersFunc

	return runProfiling(conf, ctx, logger, repo, profiling.New)
}

func runProfiling(
	conf ConfigProfiling, ctx context.Context, logger *zap.Logger,
	repo localrepo.LocalRepoIface, newProfilingersFunc profiling.NewFunc,
) error {
	logger.Info("start profiling")
	defer func() { logger.Info("finish profiling") }()
	// Load isucontinus.yaml
	isucontinuous, err := repo.LoadConf()
	if err != nil {
		return err
	}
	// Get revision
	gitRevision, err := repo.GetRevision(ctx)
	if err != nil {
		return fmt.Errorf("%s/.revision is not found. exec `deploy` command first", conf.LocalRepoPath)
	}
	// Profiling files to per host
	return perHostExec(logger, ctx, isucontinuous.Hosts, func(ctx context.Context, host config.Host) error {
		profilinger, err := newProfilingersFunc(logger, template.New(gitRevision), host)
		if err != nil {
			return err
		}
		if err := profilinger.Profiling(ctx, host.Profiling.Command); err != nil {
			return err
		}
		return nil
	})
}
