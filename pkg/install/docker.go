package install

import (
	"context"

	myerrors "github.com/ShotaKitazawa/isucontinuous/pkg/errors"
)

func (i *Installer) Docker(ctx context.Context) error {
	i.log.Info("### install Docker ###")

	// ealry return if Docker has already installed
	if stdout, stderr, err := i.runCommand(ctx, "", "which -a docker"); err != nil {
		return myerrors.NewErrorCommandExecutionFailed(stderr)
	} else if len(stdout.Bytes()) != 0 {
		i.log.Info("... Docker has already been installed")
		return nil
	}

	stdout, stderr, err := i.runCommand(ctx, "", "curl -fsSL https://get.docker.com/ | sh")
	if err != nil {
		return myerrors.NewErrorCommandExecutionFailed(stderr)
	}
	i.log.Debug(stdout.String())

	i.log.Info("... installed Docker!")
	return nil
}
