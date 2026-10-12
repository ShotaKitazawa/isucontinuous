package errors

import (
	"bytes"
	"fmt"
	"strings"
)

/* Unkouwn */

type Unkouwn struct{}

func NewErrorUnkouwn() Unkouwn {
	return Unkouwn{}
}

func (e Unkouwn) Error() string {
	return fmt.Sprintf("unknown error is occurred.")
}

/* FileAlreadyExisted */

type FileAlreadyExisted struct {
	path string
}

func NewErrorFileAlreadyExisted(path string) FileAlreadyExisted {
	return FileAlreadyExisted{path}
}

func (e FileAlreadyExisted) Error() string {
	return fmt.Sprintf("%s already existed.", e.path)
}

/* IsNotFile */

type IsNotFile struct {
	path string
}

func NewErrorIsNotFile(path string) IsNotFile {
	return IsNotFile{path}
}

func (e IsNotFile) Error() string {
	return fmt.Sprintf("%s is not file.", e.path)
}

/* IsNotDirectory */

type IsNotDirectory struct {
	path string
}

func NewErrorIsNotDirectory(path string) IsNotDirectory {
	return IsNotDirectory{path}
}

func (e IsNotDirectory) Error() string {
	return fmt.Sprintf("%s is not directory.", e.path)
}

/* CommandExecutionFailed */

type CommandExecutionFailed struct {
	message string
}

func NewErrorCommandExecutionFailed(stderr bytes.Buffer) CommandExecutionFailed {
	return CommandExecutionFailed{stderr.String()}
}

func (e CommandExecutionFailed) Error() string {
	return fmt.Sprintf("command execution failed.\n%v", e.message)
}

/* GitBranchIsDetached */

type GitBranchIsDetached struct{}

func NewErrorGitBranchIsDetached() GitBranchIsDetached {
	return GitBranchIsDetached{}
}

func (e GitBranchIsDetached) Error() string {
	return fmt.Sprintf("current branch name is <detached>. Please exec `sync` command first to checkout.")
}

/* GitBranchIsFirstCommit */

type GitBranchIsFirstCommit struct{}

func NewErrorGitBranchIsFirstCommit() GitBranchIsFirstCommit {
	return GitBranchIsFirstCommit{}
}

func (e GitBranchIsFirstCommit) Error() string {
	return fmt.Sprintf("current branch name is <detached>. Please exec `sync` command first to checkout.")
}

/* HostTask */

type HostTask struct {
	Host string
	Task string
	Err  error
}

func NewErrorHostTask(host, task string, err error) HostTask {
	return HostTask{host, task, err}
}

// Error does not prefix host/task: that would change the CLI error output.
func (e HostTask) Error() string {
	return e.Err.Error()
}

func (e HostTask) Unwrap() error {
	return e.Err
}

/* LocalRepoDirty */

type LocalRepoDirty struct {
	Paths []string
}

func NewErrorLocalRepoDirty(paths []string) LocalRepoDirty {
	return LocalRepoDirty{paths}
}

func (e LocalRepoDirty) Error() string {
	return fmt.Sprintf("local-repo has uncommitted or untracked files: %s", strings.Join(e.Paths, ", "))
}

/* LocalRepoLocked */

type LocalRepoLocked struct {
	Path string
}

func NewErrorLocalRepoLocked(path string) LocalRepoLocked {
	return LocalRepoLocked{path}
}

func (e LocalRepoLocked) Error() string {
	return fmt.Sprintf("local-repo is in use by another isucontinuous process (lock: %s).", e.Path)
}

/* RevisionMismatch */

type RevisionMismatch struct {
	Host   string
	Locked string
	Head   string
}

func NewErrorRevisionMismatch(host, locked, head string) RevisionMismatch {
	return RevisionMismatch{host, locked, head}
}

func (e RevisionMismatch) Error() string {
	return fmt.Sprintf("the lock of %s has revision %q but local-repo HEAD is %q. Please deploy again or checkout the locked revision.",
		e.Host, e.Locked, e.Head)
}
