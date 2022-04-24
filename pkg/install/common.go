package install

import (
	"github.com/ShotaKitazawa/isucontinuous/pkg/shell"
	"go.uber.org/zap"
)

type Installer struct {
	log   zap.Logger
	shell shell.Iface
}

func NewInstaller(l *zap.Logger, s shell.Iface) *Installer {
	return &Installer{*l, s}
}
