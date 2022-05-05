package cmd

import (
	"context"
	"fmt"

	"github.com/ShotaKitazawa/isucontinuous/pkg/localrepo"
	"go.uber.org/zap"
	"k8s.io/utils/exec"
)

type ConfigPush struct {
	ConfigCommon
	GitBranch string
}

func RunPush(conf ConfigPush) error {
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
	return runPush(conf, ctx, logger, repo)
}

func runPush(
	conf ConfigPush, ctx context.Context, logger *zap.Logger,
	repo localrepo.LocalRepoIface,
) error {
	logger.Info("start push")
	defer func() { logger.Info("finish push") }()
	// Check currentBranch
	var isFirstCommit = false
	currentBranch, err := repo.CurrentBranch(ctx)
	if err != nil {
		return err
	} else if currentBranch != conf.GitBranch {
		isFirstCommit, err = repo.IsFirstCommit(ctx)
		if err != nil {
			return err
		} else if isFirstCommit {
			if currentBranch == "" {
				currentBranch = "<detached>"
			}
			return fmt.Errorf(
				"current branch name is %s. Please exec `sync` command first to checkout to %s.",
				currentBranch, conf.GitBranch,
			)
		}
	}
	// Fetch
	if err := repo.Fetch(ctx); err != nil {
		return err
	}
	// Validate whether ${BRANCH} == remotes/origin/${BRANCH}
	if ok, err := repo.DiffWithRemote(ctx); err != nil && !isFirstCommit {
		return err
	} else if !ok && !isFirstCommit {
		return fmt.Errorf("there are differences between %s and remotes/origin/%s", conf.GitBranch, conf.GitBranch)
	}
	// Execute add, commit, and push
	if err := repo.Push(ctx); err != nil {
		return err
	}
	return nil
}
