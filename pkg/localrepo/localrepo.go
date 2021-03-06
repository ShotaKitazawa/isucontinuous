package localrepo

import (
	"context"
	"errors"
	"os"
	"path/filepath"

	"go.uber.org/zap"
	"gopkg.in/yaml.v3"
	"k8s.io/utils/exec"

	"github.com/ShotaKitazawa/isucontinuous/pkg/config"
	myerrors "github.com/ShotaKitazawa/isucontinuous/pkg/errors"
	"github.com/ShotaKitazawa/isucontinuous/pkg/shell"
)

const (
	isucontinuousFilename = "isucontinuous.yaml"
	revisionStoreFilename = ".revision"
)

type LocalRepoIface interface {
	LoadConf() (*config.Config, error)
	CreateFile(name string, data []byte, perm os.FileMode) error
	Fetch(ctx context.Context) error
	SwitchAndMerge(ctx context.Context, branch string) error
	SwitchDetachedBranch(ctx context.Context, revision string) error
	Push(ctx context.Context) error
	CurrentBranch(ctx context.Context) (string, error)
	IsFirstCommit(ctx context.Context) (bool, error)
	DiffWithRemote(ctx context.Context) (bool, error)
	Reset(ctx context.Context) error
	GetRevision(ctx context.Context) (string, error)
	SetRevision(ctx context.Context, revision string) error
	ClearRevision(ctx context.Context) error
	GetHeadInfo(ctx context.Context) (string, string, error)
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
	// Initialize local-repo
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
	// Generate from skelton
	f, err := config.SkeltonBytes()
	if err != nil {
		return nil, err
	}
	// Create isucontinuous.yaml to local-repo.
	if err := l.CreateFile(isucontinuousFilename, f, 0644); err != nil {
		return nil, err
	}
	// Create .gitignore (.revision is written) to local-repo.
	if err := l.CreateFile(".gitignore", []byte(revisionStoreFilename), 0644); err != nil {
		return nil, err
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

func (l *LocalRepo) LoadConf() (*config.Config, error) {
	f, err := os.ReadFile(filepath.Join(l.absPath, isucontinuousFilename))
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
	// get current branch name
	currentBranch, err := l.CurrentBranch(ctx)
	if err != nil && !errors.As(err, &myerrors.GitBranchIsDetached{}) {
		return err
	}
	// check to exist
	if _, _, err := l.shell.Exec(ctx, l.absPath, `git branch --format="%(refname:short)" | grep -e ^`+branch+`$`); err != nil {
		// checkout only
		if _, stderr, err := l.shell.Execf(ctx, l.absPath, `git checkout %s`, branch); err != nil {
			return myerrors.NewErrorCommandExecutionFailed(stderr)
		}
	} else {
		if currentBranch != branch {
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
	if _, _, err := l.shell.Exec(ctx, l.absPath, `git commit -m "commit by isucontinuous"`); err != nil {
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
	} else if stdout.String() == "" {
		return "", myerrors.NewErrorGitBranchIsDetached()
	}
	return stdout.String(), nil
}

func (l *LocalRepo) IsFirstCommit(ctx context.Context) (bool, error) {
	stdout, stderr, err := l.shell.Execf(ctx, l.absPath, "git branch -a")
	if err != nil {
		return false, myerrors.NewErrorCommandExecutionFailed(stderr)
	} else if stdout.String() != "" {
		return false, nil
	}
	return true, nil

}

func (l *LocalRepo) DiffWithRemote(ctx context.Context) (bool, error) {
	// get current branch name
	currentBranch, err := l.CurrentBranch(ctx)
	if err != nil {
		return false, err
	}
	if stdout, stderr, err := l.shell.Execf(ctx, l.absPath, "git diff origin/%s %s", currentBranch, currentBranch); err != nil {
		if isFirstCommit, _ := l.IsFirstCommit(ctx); isFirstCommit {
			return false, myerrors.NewErrorGitBranchIsFirstCommit()
		}
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
	if _, stderr, err := l.shell.Exec(ctx, l.absPath, "git clean -df"); err != nil {
		return myerrors.NewErrorCommandExecutionFailed(stderr)
	}
	return nil
}

func (l *LocalRepo) GetRevision(ctx context.Context) (string, error) {
	b, err := os.ReadFile(filepath.Join(l.absPath, revisionStoreFilename))
	return string(b), err
}

func (l *LocalRepo) SetRevision(ctx context.Context, revision string) error {
	return os.WriteFile(filepath.Join(l.absPath, revisionStoreFilename), []byte(revision), 0644)
}

func (l *LocalRepo) ClearRevision(ctx context.Context) error {
	return os.Remove(filepath.Join(l.absPath, revisionStoreFilename))
}

func (l *LocalRepo) GetHeadInfo(ctx context.Context) (string, string, error) {
	stdout, stderr, err := l.shell.Exec(ctx, l.absPath, "git rev-parse HEAD")
	if err != nil {
		return "", "", myerrors.NewErrorCommandExecutionFailed(stderr)
	}
	hash := stdout.String()
	stdout, stderr, err = l.shell.Exec(ctx, l.absPath, "git log -1 --pretty=%B")
	if err != nil {
		return "", "", myerrors.NewErrorCommandExecutionFailed(stderr)
	}
	msg := stdout.String()
	return hash, msg, nil
}
