package cmd

import (
	"context"
	"errors"

	myerrors "github.com/ShotaKitazawa/isucontinuous/pkg/errors"
	"github.com/ShotaKitazawa/isucontinuous/pkg/localrepo"
)

func syncForImport(ctx context.Context, repo localrepo.LocalRepoIface, branch string) error {
	dirty, err := repo.IsClean(ctx, false)
	if err != nil {
		return err
	}
	if len(dirty) > 0 {
		return myerrors.NewErrorLocalRepoDirty(dirty)
	}
	if _, err := repo.CurrentBranch(ctx); err != nil && errors.As(err, &myerrors.GitBranchIsDetached{}) {
		if err := repo.Reset(ctx); err != nil {
			return err
		}
	} else if err != nil {
		return err
	}
	if err := repo.Fetch(ctx); err != nil {
		return err
	}
	return repo.SwitchAndMerge(ctx, branch)
}
