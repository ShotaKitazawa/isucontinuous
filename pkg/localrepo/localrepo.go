package localrepo

import (
	"context"
	"os"
	"path/filepath"

	"go.uber.org/zap"
	"gopkg.in/yaml.v3"
	"k8s.io/utils/exec"

	"github.com/ShotaKitazawa/isu-continuous/pkg/config"
	myerrors "github.com/ShotaKitazawa/isu-continuous/pkg/errors"
	"github.com/ShotaKitazawa/isu-continuous/pkg/shell"
)

type LocalRepoIface interface {
	LoadConf(filename string) (*config.Config, error)
	CreateFile(name string, data []byte, perm os.FileMode) error
	Fetch(ctx context.Context) error
	SwitchAndMerge(ctx context.Context, branch string) error
	SwitchDetachedBranch(ctx context.Context, revision string) error
	Push(ctx context.Context) error
	CurrentBranch(ctx context.Context) (string, error)
	DiffWithRemote(ctx context.Context) (bool, error)
	Reset(ctx context.Context) error
}

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

func (l *LocalRepo) LoadConf(filename string) (*config.Config, error) {
	f, err := os.ReadFile(filepath.Join(l.absPath, filename))
	if err != nil {
		return nil, err
	}
	conf := &config.Config{}
	if err := yaml.Unmarshal(f, conf); err != nil {
		return nil, err
	}
	return conf, nil

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

func (l *LocalRepo) SwitchAndMerge(ctx context.Context, branch string) error {
	// check to exist
	if stdout, _, err := l.shell.Exec(ctx, l.absPath, `git branch --format="%(refname:short)" | grep -e ^`+branch+`$`); err != nil {
		// checkout only
		if _, stderr, err := l.shell.Execf(ctx, l.absPath, `git checkout %s`, branch); err != nil {
			return myerrors.NewErrorCommandExecutionFailed(stderr)
		}
	} else {
		if stdout.String() != branch {
			// checkout & merge
			if _, stderr, err := l.shell.Execf(ctx, l.absPath, `git checkout %s`, branch); err != nil {
				return myerrors.NewErrorCommandExecutionFailed(stderr)
			}
			if _, stderr, err := l.shell.Execf(ctx, l.absPath, `git merge origin/%s`, branch); err != nil {
				return myerrors.NewErrorCommandExecutionFailed(stderr)
			}
			return nil
		}
	}
	return nil
}

func (l *LocalRepo) SwitchDetachedBranch(ctx context.Context, revision string) error {
	if _, stderr, err := l.shell.Execf(ctx, l.absPath, "git checkout -d remotes/origin/%s || git checkout -d %s", revision, revision); err != nil {
		return myerrors.NewErrorCommandExecutionFailed(stderr)
	}
	return nil
}

func (l *LocalRepo) Push(ctx context.Context) error {
	if _, stderr, err := l.shell.Exec(ctx, l.absPath, `git add -A`); err != nil {
		return myerrors.NewErrorCommandExecutionFailed(stderr)
	}
	if _, _, err := l.shell.Exec(ctx, l.absPath, `git commit -m "commit by isu-continuous"`); err != nil {
		l.log.Info("failed `git commit`: no commit files")
	}
	if _, _, err := l.shell.Exec(ctx, l.absPath, `git push origin HEAD`); err != nil {
		l.log.Info("failed `git push`: no push commits")
	}
	return nil
}

func (l *LocalRepo) CurrentBranch(ctx context.Context) (string, error) {
	stdout, stderr, err := l.shell.Execf(ctx, l.absPath, "git branch --show-current")
	if err != nil {
		return "", myerrors.NewErrorCommandExecutionFailed(stderr)
	}
	return stdout.String(), nil
}

func (l *LocalRepo) DiffWithRemote(ctx context.Context) (bool, error) {
	if stdout, stderr, err := l.shell.Exec(ctx, l.absPath, "git diff origin/HEAD HEAD"); err != nil {
		return false, myerrors.NewErrorCommandExecutionFailed(stderr)
	} else if stdout.String() != "" {
		return false, nil
	}
	return true, nil
}

func (l *LocalRepo) Reset(ctx context.Context) error {
	if _, stderr, err := l.shell.Exec(ctx, l.absPath, "git reset --hard"); err != nil {
		return myerrors.NewErrorCommandExecutionFailed(stderr)
	}
	return nil
}
