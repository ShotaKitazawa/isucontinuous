package install

import (
	"context"
	"fmt"

	"go.uber.org/zap"

	myerrors "github.com/ShotaKitazawa/isucontinuous/pkg/errors"
	"github.com/ShotaKitazawa/isucontinuous/pkg/shell"
)

func (i *Installer) Alp(ctx context.Context, version string) error {
	i.log.Info("### install alp ###", zap.String("host", i.shell.Host()))

	// ealry return if alp has already installed
	if stdout, _, _ := i.shell.Exec(ctx, "", `which -a alp`); len(stdout.Bytes()) != 0 {
		i.log.Info("... alp has already been installed", zap.String("host", i.shell.Host()))
		return nil
	}

	url := fmt.Sprintf("https://github.com/tkuchiki/alp/releases/download/%s/alp_linux_amd64.zip", version)
	if version == "latest" {
		url = "https://github.com/tkuchiki/alp/releases/latest/download/alp_linux_amd64.zip"
	}
	command := fmt.Sprintf(
		`curl -fSL %s -o /tmp/alp.zip`, shell.Quote(url))
	if _, stderr, err := i.shell.Exec(ctx, "", command); err != nil {
		return myerrors.NewErrorCommandExecutionFailed(stderr)
	}
	i.log.Debug("downloaded to /tmp/alp.zip", zap.String("host", i.shell.Host()))

	if _, stderr, err := i.shell.Exec(ctx, "", `unzip /tmp/alp.zip -d /usr/local/bin/`); err != nil {
		return myerrors.NewErrorCommandExecutionFailed(stderr)
	}

	i.log.Info("... installed alp!", zap.String("host", i.shell.Host()))
	return nil
}
