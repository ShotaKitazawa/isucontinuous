package localrepo

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"go.uber.org/zap"
	kexec "k8s.io/utils/exec"

	"github.com/ShotaKitazawa/isucontinuous/pkg/config"
	myerrors "github.com/ShotaKitazawa/isucontinuous/pkg/errors"
	"github.com/ShotaKitazawa/isucontinuous/pkg/shell"
)

const (
	isucontinuousFilename = "isucontinuous.yaml"
	defaultCommitMessage  = "commit by isucontinuous"
	// large files are warned, not blocked: the repository is disposable and GitHub rejects >100MB itself
	largeFileThreshold = 1 << 20
	// a remote that never answers must not hang init
	remoteCheckTimeout = 30 * time.Second
)

type PushedFile struct {
	Path    string
	Size    int64
	Deleted bool
}

type PushResult struct {
	Files    []PushedFile
	Warnings []string
}

// SymlinkInWritePath reports a symlink that would be followed while writing an imported file.
type SymlinkInWritePath struct {
	Path string
}

func (e *SymlinkInWritePath) Error() string {
	return fmt.Sprintf("refusing to write through symlink %q", e.Path)
}

type LocalRepoIface interface {
	LoadConf() (*config.Config, error)
	HasDisabledHost() (bool, error)
	HasPlaintextPassword() (bool, error)
	ValidateWritePath(name string) error
	CreateFile(name string, data []byte, perm os.FileMode) error
	Fetch(ctx context.Context) error
	SwitchAndMerge(ctx context.Context, branch string) error
	SwitchDetachedBranch(ctx context.Context, revision string) error
	Push(ctx context.Context, message string) (*PushResult, error)
	CurrentBranch(ctx context.Context) (string, error)
	IsFirstCommit(ctx context.Context) (bool, error)
	DiffWithRemote(ctx context.Context) (bool, error)
	Reset(ctx context.Context) error
	GetHeadInfo(ctx context.Context) (string, string, error)
	ResolveRevision(ctx context.Context, revision string) (string, error)
	ShowFile(ctx context.Context, hash, path string) ([]byte, bool, error)
	IsClean(ctx context.Context, includeIgnored bool) ([]string, error)
	GetHeadCommit(ctx context.Context) (*CommitInfo, error)
	RemoteBranches(ctx context.Context) (branches []string, defaultBranch string, err error)
}

type CommitInfo struct {
	Hash    string
	Message string
	Date    time.Time
}

type LocalRepo struct {
	log   zap.Logger
	shell *shell.LocalClient

	absPath string
}

func InitLocalRepo(logger *zap.Logger, e kexec.Interface, path, username, email, remoteUrl, sshKey string) (*LocalRepo, []string, error) {
	return InitLocalRepoContext(context.Background(), logger, e, path, username, email, remoteUrl, sshKey)
}

func InitLocalRepoContext(ctx context.Context, logger *zap.Logger, e kexec.Interface, path, username, email, remoteUrl, sshKey string) (*LocalRepo, []string, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, nil, err
	}
	l := &LocalRepo{*logger, shell.NewLocalClient(e), absPath}
	// Initialize local-repo
	if _, stderr, err := l.shell.Exec(ctx, l.absPath, "git init"); err != nil {
		return nil, nil, myerrors.NewErrorCommandExecutionFailed(stderr)
	}
	if _, stderr, err := l.shell.Execf(ctx, l.absPath, `git config user.name "%s"`, username); err != nil {
		return nil, nil, myerrors.NewErrorCommandExecutionFailed(stderr)
	}
	if _, stderr, err := l.shell.Execf(ctx, l.absPath, `git config user.email "%s"`, email); err != nil {
		return nil, nil, myerrors.NewErrorCommandExecutionFailed(stderr)
	}
	if _, stderr, err := l.shell.Execf(ctx, l.absPath, `git remote add origin "%s"`, remoteUrl); err != nil {
		return nil, nil, myerrors.NewErrorCommandExecutionFailed(stderr)
	}
	if sshKey != "" {
		// git runs core.sshCommand through a shell, so the path is single-quoted instead of escaped
		if strings.ContainsAny(sshKey, "'\n") {
			return nil, nil, fmt.Errorf("ssh key path must not contain a single quote or newline: %q", sshKey)
		}
		sshCommand := fmt.Sprintf("ssh -i '%s' -o IdentitiesOnly=yes", sshKey)
		if _, stderr, err := l.shell.ExecArgs(ctx, l.absPath, "git", "config", "core.sshCommand", sshCommand); err != nil {
			return nil, nil, myerrors.NewErrorCommandExecutionFailed(stderr)
		}
	}
	lsCtx, cancel := context.WithTimeout(ctx, remoteCheckTimeout)
	defer cancel()
	branches, defaultBranch, err := l.RemoteBranches(lsCtx)
	if err != nil {
		// an unreachable remote must not block the first setup; the user sees it in Warnings
		warning := fmt.Sprintf("could not reach the remote, so the existing branches were not checked: %v", err)
		if err := l.createSkelton(); err != nil {
			return nil, nil, err
		}
		return l, []string{warning}, nil
	}
	if len(branches) == 0 {
		if err := l.createSkelton(); err != nil {
			return nil, nil, err
		}
		return l, nil, nil
	}
	branch := pickBranch(branches, defaultBranch)
	if _, stderr, err := l.shell.ExecArgs(ctx, l.absPath, "git", "fetch", "origin"); err != nil {
		return nil, nil, myerrors.NewErrorCommandExecutionFailed(stderr)
	}
	if _, stderr, err := l.shell.ExecArgs(ctx, l.absPath, "git", "checkout", "-B", branch, "origin/"+branch); err != nil {
		return nil, nil, myerrors.NewErrorCommandExecutionFailed(stderr)
	}
	return l, nil, nil
}

func (l *LocalRepo) createSkelton() error {
	f, err := config.SkeltonBytes()
	if err != nil {
		return err
	}
	return l.CreateFile(isucontinuousFilename, f, 0644)
}

// pickBranch prefers the branch HEAD of the remote points to; there is no branch flag on init, so master and main follow.
func pickBranch(branches []string, defaultBranch string) string {
	for _, want := range []string{defaultBranch, "master", "main"} {
		for _, b := range branches {
			if want != "" && b == want {
				return b
			}
		}
	}
	return branches[0]
}

func AttachLocalRepo(logger *zap.Logger, e kexec.Interface, path string) (*LocalRepo, error) {
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
	return config.Load(f)
}

func (l *LocalRepo) HasDisabledHost() (bool, error) {
	f, err := os.ReadFile(filepath.Join(l.absPath, isucontinuousFilename))
	if err != nil {
		return false, err
	}
	return config.HasDisabledHost(f)
}

func (l *LocalRepo) HasPlaintextPassword() (bool, error) {
	f, err := os.ReadFile(filepath.Join(l.absPath, isucontinuousFilename))
	if err != nil {
		return false, err
	}
	return config.HasPlaintextPassword(f)
}

func (l *LocalRepo) CreateFile(name string, data []byte, perm os.FileMode) error {
	if err := l.ValidateWritePath(name); err != nil {
		return err
	}
	fileAbsPath := filepath.Join(l.absPath, name)
	if _, err := os.Stat(fileAbsPath); err != nil {
		if err := os.MkdirAll(filepath.Dir(fileAbsPath), 0755); err != nil {
			return err
		}
	}
	if err := l.ValidateWritePath(name); err != nil {
		return err
	}
	return os.WriteFile(fileAbsPath, data, perm)
}

func (l *LocalRepo) ValidateWritePath(name string) error {
	clean := filepath.Clean(name)
	if clean == "." || filepath.IsAbs(name) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return fmt.Errorf("write path must stay inside local-repo: %q", name)
	}

	current := l.absPath
	for _, component := range strings.Split(clean, string(filepath.Separator)) {
		current = filepath.Join(current, component)
		info, err := os.Lstat(current)
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			rel, err := filepath.Rel(l.absPath, current)
			if err != nil {
				return err
			}
			return &SymlinkInWritePath{Path: rel}
		}
	}
	return nil
}

func (l *LocalRepo) Fetch(ctx context.Context) error {
	if _, stderr, err := l.shell.Exec(ctx, l.absPath, "git fetch"); err != nil {
		return myerrors.NewErrorCommandExecutionFailed(stderr)
	}
	return nil
}

func (l *LocalRepo) SwitchAndMerge(ctx context.Context, branch string) error {
	if err := validateRef(branch); err != nil {
		return err
	}
	// get current branch name
	currentBranch, err := l.CurrentBranch(ctx)
	if err != nil && !errors.As(err, &myerrors.GitBranchIsDetached{}) {
		return err
	}
	// check to exist
	if _, _, err := l.shell.ExecArgs(ctx, l.absPath, "git", "rev-parse", "--verify", "--quiet", "refs/heads/"+branch); err != nil {
		// checkout only
		if _, stderr, err := l.shell.ExecArgs(ctx, l.absPath, "git", "checkout", branch); err != nil {
			return myerrors.NewErrorCommandExecutionFailed(stderr)
		}
	} else {
		if currentBranch != branch {
			// checkout before merging the remote branch
			if _, stderr, err := l.shell.ExecArgs(ctx, l.absPath, "git", "checkout", branch); err != nil {
				return myerrors.NewErrorCommandExecutionFailed(stderr)
			}
		}
		if _, stderr, err := l.shell.ExecArgs(ctx, l.absPath, "git", "merge", "origin/"+branch); err != nil {
			return myerrors.NewErrorCommandExecutionFailed(stderr)
		}
		return nil
	}
	return nil
}

func (l *LocalRepo) SwitchDetachedBranch(ctx context.Context, revision string) error {
	if err := validateRef(revision); err != nil {
		return err
	}
	if _, _, err := l.shell.ExecArgs(ctx, l.absPath, "git", "checkout", "-d", "remotes/origin/"+revision); err == nil {
		return nil
	}
	if _, stderr, err := l.shell.ExecArgs(ctx, l.absPath, "git", "checkout", "-d", revision); err != nil {
		return myerrors.NewErrorCommandExecutionFailed(stderr)
	}
	return nil
}

func (l *LocalRepo) Push(ctx context.Context, message string) (*PushResult, error) {
	if message == "" {
		message = defaultCommitMessage
	}
	result := &PushResult{}
	if _, stderr, err := l.shell.Exec(ctx, l.absPath, `git add -A`); err != nil {
		return nil, myerrors.NewErrorCommandExecutionFailed(stderr)
	}
	// exit status 1 means that there are staged changes
	_, stderr, err := l.shell.ExecArgs(ctx, l.absPath, "git", "diff", "--cached", "--quiet")
	var exitErr kexec.ExitError
	if err != nil {
		if !errors.As(err, &exitErr) || exitErr.ExitStatus() != 1 {
			return nil, myerrors.NewErrorCommandExecutionFailed(stderr)
		}
		files, err := l.stagedFiles(ctx)
		if err != nil {
			return nil, err
		}
		result.Files = files
		for _, f := range files {
			if f.Size >= largeFileThreshold {
				result.Warnings = append(result.Warnings,
					fmt.Sprintf("%s is large (%d bytes)", f.Path, f.Size))
			}
		}
		if _, stderr, err := l.shell.ExecArgs(ctx, l.absPath, "git", "commit", "-m", message); err != nil {
			return nil, myerrors.NewErrorCommandExecutionFailed(stderr)
		}
	}
	if _, stderr, err := l.shell.Exec(ctx, l.absPath, `git push origin HEAD`); err != nil {
		return nil, myerrors.NewErrorCommandExecutionFailed(stderr)
	}
	return result, nil
}

func (l *LocalRepo) stagedFiles(ctx context.Context) ([]PushedFile, error) {
	stdout, stderr, err := l.shell.ExecArgs(ctx, l.absPath, "git", "diff", "--cached", "--name-only", "-z")
	if err != nil {
		return nil, myerrors.NewErrorCommandExecutionFailed(stderr)
	}
	files := []PushedFile{}
	for _, name := range strings.Split(stdout.String(), "\x00") {
		if name == "" {
			continue
		}
		info, err := os.Stat(filepath.Join(l.absPath, name))
		if os.IsNotExist(err) {
			files = append(files, PushedFile{Path: name, Deleted: true})
			continue
		} else if err != nil {
			return nil, err
		}
		files = append(files, PushedFile{Path: name, Size: info.Size()})
	}
	return files, nil
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
	currentBranch, err := l.CurrentBranch(ctx)
	if err != nil {
		return false, err
	}
	remoteRef := "refs/remotes/origin/" + currentBranch
	if _, stderr, err := l.shell.ExecArgs(ctx, l.absPath, "git", "rev-parse", "--verify", "--quiet", remoteRef); err != nil {
		if isFirstCommit, _ := l.IsFirstCommit(ctx); isFirstCommit {
			return false, myerrors.NewErrorGitBranchIsFirstCommit()
		}
		var exitErr kexec.ExitError
		if errors.As(err, &exitErr) && exitErr.ExitStatus() == 1 {
			// A local commit left by a failed first push is safe to retry.
			return true, nil
		}
		return false, myerrors.NewErrorCommandExecutionFailed(stderr)
	}
	if _, stderr, err := l.shell.ExecArgs(ctx, l.absPath, "git", "merge-base", "--is-ancestor", remoteRef, currentBranch); err != nil {
		var exitErr kexec.ExitError
		if errors.As(err, &exitErr) && exitErr.ExitStatus() == 1 {
			return false, nil
		}
		return false, myerrors.NewErrorCommandExecutionFailed(stderr)
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

// IsClean returns the paths that make the working tree dirty; ignored files count only with includeIgnored,
// because the Deployer walks the file system and would deploy them.
func (l *LocalRepo) IsClean(ctx context.Context, includeIgnored bool) ([]string, error) {
	args := []string{"status", "--porcelain", "-z", "--untracked-files=all"}
	if includeIgnored {
		args = append(args, "--ignored")
	}
	stdout, stderr, err := l.shell.ExecArgsRaw(ctx, l.absPath, "git", args...)
	if err != nil {
		return nil, myerrors.NewErrorCommandExecutionFailed(stderr)
	}
	var paths []string
	entries := strings.Split(stdout.String(), "\x00")
	for i := 0; i < len(entries); i++ {
		e := entries[i]
		if len(e) < 4 {
			continue
		}
		paths = append(paths, e[3:])
		// a rename or copy is followed by its source path
		if e[0] == 'R' || e[0] == 'C' {
			i++
		}
	}
	return paths, nil
}

func (l *LocalRepo) GetHeadCommit(ctx context.Context) (*CommitInfo, error) {
	stdout, stderr, err := l.shell.ExecArgsRaw(ctx, l.absPath, "git", "log", "-1", "--format=%H%x00%cI%x00%B")
	if err != nil {
		return nil, myerrors.NewErrorCommandExecutionFailed(stderr)
	}
	parts := strings.SplitN(stdout.String(), "\x00", 3)
	if len(parts) != 3 {
		return nil, fmt.Errorf("unexpected git log output: %q", stdout.String())
	}
	date, err := time.Parse(time.RFC3339, parts[1])
	if err != nil {
		return nil, err
	}
	return &CommitInfo{Hash: parts[0], Date: date, Message: strings.TrimSpace(parts[2])}, nil
}

// RemoteBranches lists the branches of origin and the one HEAD points to, if any.
func (l *LocalRepo) RemoteBranches(ctx context.Context) ([]string, string, error) {
	stdout, stderr, err := l.shell.ExecArgs(ctx, l.absPath, "git", "ls-remote", "--symref", "origin", "HEAD", "refs/heads/*")
	if err != nil {
		return nil, "", myerrors.NewErrorCommandExecutionFailed(stderr)
	}
	var branches []string
	defaultBranch := ""
	for _, line := range strings.Split(stdout.String(), "\n") {
		fields := strings.Fields(line)
		switch {
		case len(fields) == 3 && fields[0] == "ref:" && fields[2] == "HEAD":
			defaultBranch = strings.TrimPrefix(fields[1], "refs/heads/")
		case len(fields) == 2 && strings.HasPrefix(fields[1], "refs/heads/"):
			branches = append(branches, strings.TrimPrefix(fields[1], "refs/heads/"))
		}
	}
	return branches, defaultBranch, nil
}

// Lock excludes other processes on this machine from operating on local-repo; it does not wait.
func (l *LocalRepo) Lock(ctx context.Context) (func(), error) {
	stdout, stderr, err := l.shell.ExecArgs(ctx, l.absPath, "git", "rev-parse", "--absolute-git-dir")
	if err != nil {
		return nil, myerrors.NewErrorCommandExecutionFailed(stderr)
	}
	return lockFile(filepath.Join(stdout.String(), "isucontinuous.lock"))
}

// ResolveRevision resolves revision to a commit hash without moving the working tree.
// The remote-tracking branch is tried first, like SwitchDetachedBranch.
func (l *LocalRepo) ResolveRevision(ctx context.Context, revision string) (string, error) {
	if err := validateRef(revision); err != nil {
		return "", err
	}
	for _, ref := range []string{"remotes/origin/" + revision, revision} {
		if stdout, _, err := l.shell.ExecArgs(ctx, l.absPath, "git", "rev-parse", "--verify", "--quiet", ref+"^{commit}"); err == nil {
			return stdout.String(), nil
		}
	}
	return "", fmt.Errorf("unknown revision: %q", revision)
}

// ShowFile returns the content of path at the commit hash; found is false if the commit has no such file.
func (l *LocalRepo) ShowFile(ctx context.Context, hash, path string) (content []byte, found bool, err error) {
	if err := validateRef(hash); err != nil {
		return nil, false, err
	}
	object := hash + ":" + path
	if _, _, err := l.shell.ExecArgs(ctx, l.absPath, "git", "cat-file", "-e", object); err != nil {
		return nil, false, nil
	}
	stdout, stderr, err := l.shell.ExecArgsRaw(ctx, l.absPath, "git", "show", object)
	if err != nil {
		return nil, false, myerrors.NewErrorCommandExecutionFailed(stderr)
	}
	return stdout.Bytes(), true, nil
}

// validateRef follows git check-ref-format; revision expressions such as HEAD~1 are rejected on purpose.
func validateRef(ref string) error {
	invalid := func() error { return fmt.Errorf("invalid git ref name: %q", ref) }
	if ref == "" || ref == "@" || strings.HasPrefix(ref, "-") ||
		strings.HasPrefix(ref, "/") || strings.HasSuffix(ref, "/") ||
		strings.HasSuffix(ref, ".") || strings.Contains(ref, "..") ||
		strings.Contains(ref, "//") || strings.Contains(ref, "@{") {
		return invalid()
	}
	for _, r := range ref {
		if r < 0x20 || r == 0x7f || strings.ContainsRune(" ~^:?*[\\", r) {
			return invalid()
		}
	}
	for _, c := range strings.Split(ref, "/") {
		if strings.HasPrefix(c, ".") || strings.HasSuffix(c, ".lock") {
			return invalid()
		}
	}
	return nil
}
