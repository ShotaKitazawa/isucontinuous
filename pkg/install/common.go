package install

import (
	"go.uber.org/zap"

	"github.com/ShotaKitazawa/isucontinuous/pkg/shell"
)

type Installer struct {
	log   zap.Logger
	shell shell.Iface
}

func NewInstaller(l *zap.Logger, s shell.Iface) *Installer {
	return &Installer{*l, s}
}
