package install

import (
	"context"

	"go.uber.org/zap"

	myerrors "github.com/ShotaKitazawa/isucontinuous/pkg/errors"
)

func (i *Installer) Docker(ctx context.Context) error {
	i.log.Info("### install Docker ###", zap.String("host", i.shell.Host()))

	// ealry return if Docker has already installed
	if stdout, _, _ := i.shell.Exec(ctx, "", "which -a docker"); len(stdout.Bytes()) != 0 {
		i.log.Info("... Docker has already been installed", zap.String("host", i.shell.Host()))
		return nil
	}

	stdout, stderr, err := i.shell.Exec(ctx, "", "curl -fsSL https://get.docker.com/ | sh")
	if err != nil {
		return myerrors.NewErrorCommandExecutionFailed(stderr)
	}
	i.log.Debug(stdout.String(), zap.String("host", i.shell.Host()))

	i.log.Info("... installed Docker!", zap.String("host", i.shell.Host()))
	return nil
}
