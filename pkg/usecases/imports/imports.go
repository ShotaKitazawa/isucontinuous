package imports

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"go.uber.org/zap"

	myerrors "github.com/ShotaKitazawa/isu-continuous/pkg/errors"
	"github.com/ShotaKitazawa/isu-continuous/pkg/shell"
)

type Importer struct {
	log   zap.Logger
	shell shell.Iface
}

func New(logger *zap.Logger, s shell.Iface) *Importer {
	return &Importer{*logger, s}
}

const (
	IsNotFound = iota
	IsFile
	IsDirectory
)

func (l *Importer) FileType(ctx context.Context, path string) int {
	if _, _, err := l.shell.Execf(ctx, "", "test ! -f %s", path); err != nil {
		return IsFile
	}
	if _, _, err := l.shell.Execf(ctx, "", "test ! -d %s", path); err != nil {
		return IsDirectory
	}
	return IsNotFound
}

func (l *Importer) GetFileContent(ctx context.Context, path string) ([]byte, os.FileMode, error) {
	if _, _, err := l.shell.Execf(ctx, "", "test -f %s", path); err != nil {
		return nil, 0, myerrors.NewErrorIsNotFile(path)
	}
	stdout, stderr, err := l.shell.Execf(ctx, "", "cat %s", path)
	if err != nil {
		return nil, 0, myerrors.NewErrorCommandExecutionFailed(stderr)
	}
	content := stdout.Bytes()
	stdout, stderr, err = l.shell.Exec(ctx, "", "stat "+path+" -c '%a'")
	if err != nil {
		return nil, 0, myerrors.NewErrorCommandExecutionFailed(stderr)
	}
	mode, err := strconv.Atoi(stdout.String())
	if err != nil {
		return nil, 0, err
	}
	return content, os.FileMode(mode), nil
}

func (l *Importer) ListUntrackedFiles(ctx context.Context, path string) ([]string, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	if _, _, err := l.shell.Execf(ctx, "", "test -d %s", path); err != nil {
		return nil, myerrors.NewErrorIsNotDirectory(path)
	}

	if _, stderr, err := l.shell.Exec(ctx, absPath, "git init"); err != nil {
		return nil, myerrors.NewErrorCommandExecutionFailed(stderr)
	}
	defer func() {
		_, _, _ = l.shell.Execf(context.Background(), "", `rm -rf "%s"`, filepath.Join(absPath, ".git"))
	}()
	if _, stderr, err := l.shell.Execf(ctx, absPath, "git config --global --add safe.directory %s", absPath); err != nil {
		return nil, myerrors.NewErrorCommandExecutionFailed(stderr)
	}

	stdout, stderr, err := l.shell.Exec(ctx, absPath, "git ls-files --others --exclude-standard")
	if err != nil {
		return nil, myerrors.NewErrorCommandExecutionFailed(stderr)
	}
	return strings.Split(stdout.String(), "\n"), nil
}
