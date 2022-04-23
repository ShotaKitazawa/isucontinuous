package install

import (
	"context"

	myerrors "github.com/ShotaKitazawa/isucontinuous/pkg/errors"
)

func (i *Installer) Docker(ctx context.Context) error {
	stdout, stderr, err := i.runCommand(ctx, "", "curl -fsSL https://get.docker.com/ | sh")
	if err != nil {
		return myerrors.NewErrorCommandExecutionFailed(stderr)
	}
	i.log.Debug(stdout.String())
	return nil
}
