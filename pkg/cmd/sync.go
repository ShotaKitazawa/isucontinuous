package cmd

import (
	"context"

	"go.uber.org/zap"
	"k8s.io/utils/exec"

	"github.com/ShotaKitazawa/isu-continuous/pkg/localrepo"
)

type ConfigSync struct {
	ConfigCommon
	GitBranch string
}

func RunSync(conf ConfigSync) error {
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
	return runSync(conf, ctx, logger, repo)
}

func runSync(
	conf ConfigSync, ctx context.Context, logger *zap.Logger,
	repo localrepo.LocalRepoIface,
) error {
	// if current branch is detached, exec `git reset --hard``
	if currentBranch, err := repo.CurrentBranch(ctx); err != nil {
		return err
	} else if currentBranch == "" {
		if err := repo.Reset(ctx); err != nil {
			return err
		}
	}
	// Fetch remote-repo & switch to gitBranch
	if err := repo.Fetch(ctx); err != nil {
		return err
	}
	if err := repo.SwitchAndMerge(ctx, conf.GitBranch); err != nil {
		return err
	}
	return nil
}
