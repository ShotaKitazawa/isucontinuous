package gitcommand

import (
	"bytes"
	"context"
	"os"
	"path/filepath"

	"go.uber.org/zap"
	"k8s.io/utils/exec"

	myerrors "github.com/ShotaKitazawa/isucontinuous/pkg/errors"
)

type LocalRepo struct {
	log  zap.Logger
	exec exec.Interface

	path string
}

func NewLocalRepo(l *zap.Logger, e exec.Interface, path, username, email, remoteUrl string) (*LocalRepo, error) {
	if _, err := os.Stat(path); err == nil {
		return nil, myerrors.NewErrorFileAlreadyExisted(path)
	}
	if err := os.Mkdir(path, 0755); err != nil {
		return nil, err
	}
	localRepo := &LocalRepo{*l, e, path}
	ctx := context.Background()

	if _, stderr, err := localRepo.runCommand(ctx, localRepo.path, "git", "init"); err != nil {
		return nil, myerrors.NewErrorCommandExecutionFailed(stderr)
	}
	if _, stderr, err := localRepo.runCommand(ctx, localRepo.path, "git", "config", "user.name", username); err != nil {
		return nil, myerrors.NewErrorCommandExecutionFailed(stderr)
	}
	if _, stderr, err := localRepo.runCommand(ctx, localRepo.path, "git", "config", "user.email", email); err != nil {
		return nil, myerrors.NewErrorCommandExecutionFailed(stderr)
	}
	if _, stderr, err := localRepo.runCommand(ctx, localRepo.path, "git", "remote", "add", "origin", remoteUrl); err != nil {
		return nil, myerrors.NewErrorCommandExecutionFailed(stderr)
	}

	return localRepo, nil
}

func AttachLocalRepo(l *zap.Logger, e exec.Interface, path string) (*LocalRepo, error) {
	if f, err := os.Stat(path); err != nil {
		return nil, myerrors.NewErrorFileAlreadyExisted(path)
	} else if !f.IsDir() {
		return nil, myerrors.NewErrorIsNotDirectory(path)
	}
	return &LocalRepo{*l, e, path}, nil
}

func (l *LocalRepo) CreateFile(name string, data []byte, perm os.FileMode) error {
	return os.WriteFile(filepath.Join(l.path, name), data, perm)
}

func (l *LocalRepo) runCommand(ctx context.Context, basedir string, cmd string, args ...string) (bytes.Buffer, bytes.Buffer, error) {
	stdout := bytes.Buffer{}
	stderr := bytes.Buffer{}

	cc := l.exec.CommandContext(ctx, cmd, args...)
	if basedir != "" {
		cc.SetDir(basedir)
	}
	cc.SetStdout(&stdout)
	cc.SetStderr(&stderr)
	if err := cc.Run(); err != nil {
		return stdout, stderr, err
	}
	return stdout, stderr, nil
}
