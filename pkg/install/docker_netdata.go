package install

import (
	"context"
	"fmt"

	myerrors "github.com/ShotaKitazawa/isucontinuous/pkg/errors"
)

const (
	containerName = "netdata"
)

func (i *Installer) Netdata(ctx context.Context, version string, publicPort int) error {
	i.log.Info("### install Netdata ###")

	// ealry return if netdata has already installed
	command := fmt.Sprintf(
		"docker container ps -f name=%s --format {{.ID}}",
		containerName)
	if stdout, _, _ := i.runCommand(ctx, "", command); len(stdout.Bytes()) != 0 {
		i.log.Info("... Netdata has already been installed")
		return nil
	}

	command = fmt.Sprintf(`
docker run -itd -p %d:19999 \
  -v netdataconfig:/etc/netdata \
  -v netdatalib:/var/lib/netdata \
  -v netdatacache:/var/cache/netdata \
  -v /etc/passwd:/host/etc/passwd:ro \
  -v /etc/group:/host/etc/group:ro \
  -v /proc:/host/proc:ro \
  -v /sys:/host/sys:ro \
  -v /etc/os-release:/host/etc/os-release:ro \
  --restart unless-stopped \
  --cap-add SYS_PTRACE \
  --security-opt apparmor=unconfined \
  --name=%s \
  netdata/netdata:%s`, publicPort, containerName, version)
	stdout, stderr, err := i.runCommand(ctx, "", command)
	if err != nil {
		return myerrors.NewErrorCommandExecutionFailed(stderr)
	}
	i.log.Debug(stdout.String())

	i.log.Info("... installed Netdata!")
	return nil
}
