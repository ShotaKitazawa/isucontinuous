package install

import (
	"context"
	"fmt"

	myerrors "github.com/ShotaKitazawa/isucontinuous/pkg/errors"
)

func (i *Installer) Alp(ctx context.Context, version string) error {
	i.log.Info("### install alp ###")

	// ealry return if alp has already installed
	if stdout, _, _ := i.runCommand(ctx, "", "which -a alp"); len(stdout.Bytes()) != 0 {
		i.log.Info("... alp has already been installed")
		return nil
	}

	if version == "latest" {
		// TODO
		// get release
		// get latest tag
	}
	command := fmt.Sprintf(
		"curl -sL https://github.com/tkuchiki/alp/releases/download/%s/alp_linux_amd64.zip -o /tmp/alp.zip",
		version)
	stdout, stderr, err := i.runCommand(ctx, "", command)
	if err != nil {
		return myerrors.NewErrorCommandExecutionFailed(stderr)
	}
	i.log.Debug(stdout.String())

	command = "unzip -f /tmp/alp -d /usr/local/bin"
	stdout, stderr, err = i.runCommand(ctx, "", command)
	if err != nil {
		return myerrors.NewErrorCommandExecutionFailed(stderr)
	}
	i.log.Debug(stdout.String())

	i.log.Info("... installed alp!")
	return nil
}
