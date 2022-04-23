package errors

import (
	"bytes"
	"fmt"
)

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
