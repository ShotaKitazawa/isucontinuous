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
	// Force syncs even if local-repo has uncommitted changes, which are lost when HEAD is detached.
	Force bool
}

func RunSync(conf ConfigSync) (*SyncResult, error) {
	return RunSyncContext(context.Background(), conf)
}

func RunSyncContext(ctx context.Context, conf ConfigSync) (*SyncResult, error) {
	logger, err := newLogger(conf.LogLevel, conf.LogFilename)
	if err != nil {
		return nil, err
	}
	// Attach local-repo
	repo, err := localrepo.AttachLocalRepo(logger, exec.New(), conf.LocalRepoPath)
	if err != nil {
		return nil, err
	}
	unlock, err := repo.Lock(ctx)
	if err != nil {
		return nil, err
	}
	defer unlock()
	return runSync(conf, ctx, logger, repo)
}

func runSync(
	conf ConfigSync, ctx context.Context, logger *zap.Logger,
	repo localrepo.LocalRepoIface,
) (*SyncResult, error) {
	logger.Info("start sync")
	defer func() { logger.Info("finish sync") }()
	if !conf.Force {
		dirty, err := repo.IsClean(ctx, false)
		if err != nil {
			return nil, err
		}
		if len(dirty) > 0 {
			return nil, myerrors.NewErrorLocalRepoDirty(dirty)
		}
	}
	// if current branch is detached, exec `git reset --hard``
	if _, err := repo.CurrentBranch(ctx); err != nil && errors.As(err, &myerrors.GitBranchIsDetached{}) {
		if err := repo.Reset(ctx); err != nil {
			return nil, err
		}
	} else if err != nil {
		return nil, err
	}
	// Fetch remote-repo & switch to gitBranch
	if err := repo.Fetch(ctx); err != nil {
		return nil, err
	}
	if err := repo.SwitchAndMerge(ctx, conf.GitBranch); err != nil {
		return nil, err
	}
	return &SyncResult{}, nil
}
