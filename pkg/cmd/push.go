package cmd

import (
	"context"
	"fmt"

	myerrors "github.com/ShotaKitazawa/isucontinuous/pkg/errors"
	"github.com/ShotaKitazawa/isucontinuous/pkg/localrepo"
)

func pushForImport(ctx context.Context, repo localrepo.LocalRepoIface, branch, message string) (*localrepo.PushResult, error) {
	currentBranch, err := repo.CurrentBranch(ctx)
	if err != nil {
		return nil, err
	}
	if currentBranch != branch {
		if currentBranch == "" {
			currentBranch = "<detached>"
		}
		return nil, fmt.Errorf("current branch name is %s; expected %s", currentBranch, branch)
	}
	if err := repo.Fetch(ctx); err != nil {
		return nil, err
	}
	if ok, err := repo.DiffWithRemote(ctx); err != nil {
		switch err.(type) {
		case myerrors.GitBranchIsFirstCommit:
			// pass
		default:
			return nil, err
		}
	} else if !ok {
		return nil, fmt.Errorf("there are differences between %s and remotes/origin/%s", branch, branch)
	}
	pushed, err := repo.Push(ctx, message)
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
	if plain, err := repo.HasPlaintextPassword(); err != nil {
		warnings = append(warnings, fmt.Sprintf("could not check for plaintext passwords: %v", err))
	} else if plain {
		warnings = append(warnings, "isucontinuous.yaml has a plaintext password: pass it through a template such as {{.Env.X}} or use a key")
	}
	pushed.Warnings = warnings
	return pushed, nil
}
