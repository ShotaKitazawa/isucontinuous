package install

import (
	"go.uber.org/zap"
	"k8s.io/utils/exec"

	"github.com/ShotaKitazawa/isu-continuous/pkg/config"
	"github.com/ShotaKitazawa/isu-continuous/pkg/shell"
)

type Installer struct {
	log   zap.Logger
	shell shell.Iface
}

type NewInstallersFunc func(logger *zap.Logger, hosts []config.Host) (map[string]*Installer, error)

func NewInstallers(logger *zap.Logger, hosts []config.Host) (map[string]*Installer, error) {
	installers := make(map[string]*Installer)
	var err error
	for _, host := range hosts {
		var s shell.Iface
		if host.IsLocal() {
			s = shell.NewLocalClient(exec.New())
		} else {
			s, err = shell.NewSshClient(host.Host, host.Port, host.User, host.Password, host.Key)
			if err != nil {
				return nil, err
			}
		}
		installers[host.Host] = new(logger, s)
	}
	return installers, nil
}

func new(l *zap.Logger, s shell.Iface) *Installer {
	return &Installer{*l, s}
}
