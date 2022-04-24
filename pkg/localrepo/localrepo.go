package localrepo

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"go.uber.org/zap"
	"k8s.io/utils/exec"

	myerrors "github.com/ShotaKitazawa/isucontinuous/pkg/errors"
	"github.com/ShotaKitazawa/isucontinuous/pkg/shell"
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

	if _, stderr, err := l.shell.RunCommand(ctx, l.absPath, "git init"); err != nil {
		return nil, myerrors.NewErrorCommandExecutionFailed(stderr)
	}
	if _, stderr, err := l.shell.RunCommand(ctx, l.absPath, fmt.Sprintf(`git config user.name "%s"`, username)); err != nil {
		return nil, myerrors.NewErrorCommandExecutionFailed(stderr)
	}
	if _, stderr, err := l.shell.RunCommand(ctx, l.absPath, fmt.Sprintf(`git config user.email "%s"`, email)); err != nil {
		return nil, myerrors.NewErrorCommandExecutionFailed(stderr)
	}
	if _, stderr, err := l.shell.RunCommand(ctx, l.absPath, fmt.Sprintf(`git remote add origin "%s"`, remoteUrl)); err != nil {
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

func (l *LocalRepo) ListUntrackedFiles(ctx context.Context) ([]string, error) {
	stdout, stderr, err := l.shell.RunCommand(ctx, l.absPath, "git ls-files --others --exclude-standard")
	if err != nil {
		return nil, myerrors.NewErrorCommandExecutionFailed(stderr)
	}
	return strings.Split(stdout.String(), "\n"), nil
}

func (l *LocalRepo) Clear() {
	_, _, _ = l.shell.RunCommand(context.Background(), "", fmt.Sprintf(`rm -rf "%s"`, filepath.Join(l.absPath, ".git")))
}
