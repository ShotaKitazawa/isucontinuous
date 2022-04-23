package install

import (
	"bytes"
	"context"

	"go.uber.org/zap"
	"k8s.io/utils/exec"
)

type Installer struct {
	log  zap.Logger
	exec exec.Interface
}

func NewInstaller(l *zap.Logger, e exec.Interface) *Installer {
	return &Installer{*l, e}
}

func (i *Installer) runCommand(ctx context.Context, basedir string, cmd string) (bytes.Buffer, bytes.Buffer, error) {
	stdout := bytes.Buffer{}
	stderr := bytes.Buffer{}

	cc := i.exec.CommandContext(ctx, "sh", "-c", cmd)
	if basedir != "" {
		cc.SetDir(basedir)
	}
	cc.SetStdout(&stdout)
	cc.SetStderr(&stderr)
	if err := cc.Run(); err != nil {
		return stdout, stderr, err
	}
	return stdout, stderr, nil
}
