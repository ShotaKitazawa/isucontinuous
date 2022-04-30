package cmd

import (
	"context"
	"fmt"

	"github.com/ShotaKitazawa/isu-continuous/pkg/localrepo"
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
	currentBranch, err := repo.CurrentBranch(ctx)
	if err != nil {
		return err
	} else if currentBranch != conf.GitBranch {
		return fmt.Errorf(
			"current branch name is %s, exec `isu-continuous sync` command to checkout to %s.",
			currentBranch, conf.GitBranch,
		)
	}
	// Fetch
	if err := repo.Fetch(ctx); err != nil {
		return err
	}
	// Validate whether ${BRANCH} == remotes/origin/${BRANCH}
	if ok, err := repo.DiffWithRemote(ctx); err != nil {
		return err
	} else if !ok {
		return fmt.Errorf("there are deferences between %s and remotes/origin/%s", conf.GitBranch, conf.GitBranch)
	}
	// Execute add, commit, and push
	if err := repo.Push(ctx); err != nil {
		return err
	}
	return nil
}
