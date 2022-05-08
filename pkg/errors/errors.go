package errors

import (
	"bytes"
	"fmt"
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
