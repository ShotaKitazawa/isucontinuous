package cmd

import (
	"context"
	"fmt"

	"go.uber.org/zap"
	"k8s.io/utils/exec"

	myerrors "github.com/ShotaKitazawa/isucontinuous/pkg/errors"
	"github.com/ShotaKitazawa/isucontinuous/pkg/localrepo"
)

type ConfigPush struct {
	ConfigCommon
	GitBranch string
	Message   string
}

func RunPush(conf ConfigPush) (*PushResult, error) {
	ctx := context.Background()
	logger, err := newLogger(conf.LogLevel, conf.LogFilename)
	if err != nil {
		return nil, err
	}
	// Attach local-repo
	repo, err := localrepo.AttachLocalRepo(logger, exec.New(), conf.LocalRepoPath)
	if err != nil {
		return nil, err
	}
	return runPush(conf, ctx, logger, repo)
}

func runPush(
	conf ConfigPush, ctx context.Context, logger *zap.Logger,
	repo localrepo.LocalRepoIface,
) (*PushResult, error) {
	logger.Info("start push")
	defer func() { logger.Info("finish push") }()
	// Check currentBranch
	currentBranch, err := repo.CurrentBranch(ctx)
	if err != nil {
		return nil, err
	} else if currentBranch != conf.GitBranch {
		if currentBranch == "" {
			currentBranch = "<detached>"
		}
		return nil, fmt.Errorf(
			"current branch name is %s. Please exec `sync` command first to checkout to %s.",
			currentBranch, conf.GitBranch,
		)
	}
	// Fetch
	if err := repo.Fetch(ctx); err != nil {
		return nil, err
	}
	// Validate whether ${BRANCH} == remotes/origin/${BRANCH}
	if ok, err := repo.DiffWithRemote(ctx); err != nil {
		switch err.(type) {
		case myerrors.GitBranchIsFirstCommit:
			// pass
		default:
			return nil, err
		}
	} else if !ok {
		return nil, fmt.Errorf("there are differences between %s and remotes/origin/%s", conf.GitBranch, conf.GitBranch)
	}
	// Execute add, commit, and push
	pushed, err := repo.Push(ctx, conf.Message)
	if err != nil {
		return nil, err
	}
	warnings := pushed.Warnings
	if currentBranch == "main" || currentBranch == "master" {
		if disabled, err := repo.HasDisabledHost(); err != nil {
			warnings = append(warnings, fmt.Sprintf("could not check for disabled hosts: %v", err))
		} else if disabled {
			warnings = append(warnings, fmt.Sprintf(
				"isucontinuous.yaml has a host with disabled: true on %s: enable all hosts on this branch", currentBranch))
		}
	}
	return &PushResult{
		Result: Result{Warnings: warnings},
		Files:  pushed.Files,
	}, nil
}
