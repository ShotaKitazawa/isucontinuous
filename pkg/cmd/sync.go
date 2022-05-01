package cmd

import (
	"context"
	"errors"

	"go.uber.org/zap"
	"k8s.io/utils/exec"

	myerrors "github.com/ShotaKitazawa/isucontinuous/pkg/errors"
	"github.com/ShotaKitazawa/isucontinuous/pkg/localrepo"
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
	logger.Info("start sync")
	defer func() { logger.Info("finish sync") }()
	// if current branch is detached, exec `git reset --hard``
	if _, err := repo.CurrentBranch(ctx); err != nil && errors.As(err, &myerrors.GitBranchIsDetached{}) {
		if err := repo.Reset(ctx); err != nil {
			return err
		}
	} else if err != nil {
		return err
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
