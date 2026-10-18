package imports

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"go.uber.org/zap"
	"k8s.io/utils/exec"

	"github.com/ShotaKitazawa/isucontinuous/pkg/config"
	myerrors "github.com/ShotaKitazawa/isucontinuous/pkg/errors"
	"github.com/ShotaKitazawa/isucontinuous/pkg/shell"
)

type Importer struct {
	log   zap.Logger
	shell shell.Iface
}

type NewImportersFunc func(logger *zap.Logger, hosts []config.Host) (map[string]*Importer, error)

func NewImporters(logger *zap.Logger, hosts []config.Host) (map[string]*Importer, error) {
	importers := make(map[string]*Importer)
	var err error
	for _, host := range hosts {
		var s shell.Iface
		if host.Local {
			s = shell.NewLocalClient(exec.New())
		} else {
			s, err = shell.NewSshClient(host.Host, host.Port, host.User, host.Password, host.Key)
			if err != nil {
				return nil, err
			}
		}
		importers[host.Name] = new(logger, s)
	}
	return importers, nil
}

func new(logger *zap.Logger, s shell.Iface) *Importer {
	return &Importer{*logger, s}
}

const (
	IsNotFound = iota
	IsFile
	IsDirectory
)

func (l *Importer) FileType(ctx context.Context, path string) int {
	if _, _, err := l.shell.Execf(ctx, "", `test ! -f "%s"`, path); err != nil {
		return IsFile
	}
	if _, _, err := l.shell.Execf(ctx, "", `test ! -d "%s"`, path); err != nil {
		return IsDirectory
	}
	return IsNotFound
}

func (l *Importer) GetFileContent(ctx context.Context, path string) ([]byte, os.FileMode, error) {
	if _, _, err := l.shell.Execf(ctx, "", `test -f "%s"`, path); err != nil {
		return nil, 0, myerrors.NewErrorIsNotFile(path)
	}
	stdout, stderr, err := l.shell.Execf(ctx, "", `cat "%s"`, path)
	if err != nil {
		return nil, 0, myerrors.NewErrorCommandExecutionFailed(stderr)
	}
	content := stdout.Bytes()
	stdout, stderr, err = l.shell.Exec(ctx, "", "stat "+path+" -c '%a'")
	if err != nil {
		return nil, 0, myerrors.NewErrorCommandExecutionFailed(stderr)
	}
	mode, err := parseFileMode(stdout.String())
	if err != nil {
		return nil, 0, err
	}
	return content, mode, nil
}

func parseFileMode(s string) (os.FileMode, error) {
	mode, err := strconv.ParseUint(strings.TrimSpace(s), 8, 32)
	if err != nil {
		return 0, err
	}
	return os.FileMode(mode), nil
}

func (l *Importer) ListUntrackedFiles(ctx context.Context, path string) ([]string, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	if _, _, err := l.shell.Execf(ctx, "", `test -d "%s"`, path); err != nil {
		return nil, myerrors.NewErrorIsNotDirectory(path)
	}

	gitCmd := "git"
	stdout, _, err := l.shell.Exec(ctx, absPath, `git -c safe.directory='*' rev-parse --show-toplevel`)
	if err == nil {
		repoRoot := strings.TrimSpace(stdout.String())
		gitCmd = fmt.Sprintf(`git -c safe.directory=%s`, shell.Quote(repoRoot))
	} else {
		// a throwaway git-dir keeps the target free of .git; git init in the target would delete an existing history on cleanup
		tmpOut, stderr, err := l.shell.Exec(ctx, "", `mktemp -d`)
		if err != nil {
			return nil, myerrors.NewErrorCommandExecutionFailed(stderr)
		}
		gitDir := strings.TrimSpace(tmpOut.String())
		defer func() {
			_, _, _ = l.shell.Exec(context.Background(), "", `rm -rf `+shell.Quote(gitDir))
		}()
		gitCmd = fmt.Sprintf(`git --git-dir=%s --work-tree=%s`, shell.Quote(gitDir), shell.Quote(absPath))
		if _, stderr, err := l.shell.Exec(ctx, absPath, gitCmd+` init`); err != nil {
			return nil, myerrors.NewErrorCommandExecutionFailed(stderr)
		}
	}

	stdout, stderr, err := l.shell.Exec(ctx, absPath, gitCmd+` ls-files --others --exclude-standard`)
	if err != nil {
		return nil, myerrors.NewErrorCommandExecutionFailed(stderr)
	}
	files := []string{}
	for _, f := range strings.Split(stdout.String(), "\n") {
		if f != "" {
			files = append(files, f)
		}
	}
	return files, nil
}

func (l *Importer) ExcludeSymlinkFiles(ctx context.Context, files []string) []string {
	result := []string{}
	for _, f := range files {
		if _, _, err := l.shell.Execf(ctx, "", `test -L %s`, f); err != nil {
			result = append(result, f)
		}
	}
	return result
}
