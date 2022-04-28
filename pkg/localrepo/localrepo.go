package localrepo

import (
	"context"
	"os"
	"path/filepath"

	"go.uber.org/zap"
	"k8s.io/utils/exec"

	myerrors "github.com/ShotaKitazawa/isu-continuous/pkg/errors"
	"github.com/ShotaKitazawa/isu-continuous/pkg/shell"
)

type LocalRepo struct {
	log   zap.Logger
	shell *shell.LocalClient

	absPath string
}

func InitLocalRepo(logger *zap.Logger, e exec.Interface, path, username, email, remoteUrl string) (*LocalRepo, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	l := &LocalRepo{*logger, shell.NewLocalClient(e), absPath}
	ctx := context.Background()

	if _, stderr, err := l.shell.Exec(ctx, l.absPath, "git init"); err != nil {
		return nil, myerrors.NewErrorCommandExecutionFailed(stderr)
	}
	if _, stderr, err := l.shell.Execf(ctx, l.absPath, `git config user.name "%s"`, username); err != nil {
		return nil, myerrors.NewErrorCommandExecutionFailed(stderr)
	}
	if _, stderr, err := l.shell.Execf(ctx, l.absPath, `git config user.email "%s"`, email); err != nil {
		return nil, myerrors.NewErrorCommandExecutionFailed(stderr)
	}
	if _, stderr, err := l.shell.Execf(ctx, l.absPath, `git remote add origin "%s"`, remoteUrl); err != nil {
		return nil, myerrors.NewErrorCommandExecutionFailed(stderr)
	}

	return l, nil
}

func AttachLocalRepo(logger *zap.Logger, e exec.Interface, path string) (*LocalRepo, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	if f, err := os.Stat(absPath); err != nil {
		return nil, myerrors.NewErrorFileAlreadyExisted(absPath)
	} else if !f.IsDir() {
		return nil, myerrors.NewErrorIsNotDirectory(absPath)
	}
	return &LocalRepo{*logger, shell.NewLocalClient(e), absPath}, nil
}

func (l *LocalRepo) CreateFile(name string, data []byte, perm os.FileMode) error {
	fileAbsPath := filepath.Join(l.absPath, name)
	if _, err := os.Stat(fileAbsPath); err != nil {
		if err := os.MkdirAll(filepath.Dir(fileAbsPath), 0755); err != nil {
			return err
		}
	}
	return os.WriteFile(filepath.Join(l.absPath, name), data, perm)
}

func (l *LocalRepo) Fetch(ctx context.Context) error {
	if _, stderr, err := l.shell.Exec(ctx, l.absPath, "git fetch"); err != nil {
		return myerrors.NewErrorCommandExecutionFailed(stderr)
	}
	return nil
}

func (l *LocalRepo) SwitchDetachedBranch(ctx context.Context, revision string) error {
	if _, stderr, err := l.shell.Execf(ctx, l.absPath, "git checkout -d remotes/origin/%s || git checkout -d %s", revision, revision); err != nil {
		return myerrors.NewErrorCommandExecutionFailed(stderr)
	}
	return nil
}
