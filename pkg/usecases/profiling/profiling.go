package profiling

import (
	"context"

	"go.uber.org/zap"
	"k8s.io/utils/exec"

	"github.com/ShotaKitazawa/isu-continuous/pkg/config"
	myerrros "github.com/ShotaKitazawa/isu-continuous/pkg/errors"
	"github.com/ShotaKitazawa/isu-continuous/pkg/shell"
	"github.com/ShotaKitazawa/isu-continuous/pkg/template"
)

type Profilinger struct {
	log      *zap.Logger
	shell    shell.Iface
	template *template.Templator
}

type NewFunc func(*zap.Logger, *template.Templator, config.Host) (*Profilinger, error)

func New(logger *zap.Logger, templator *template.Templator, host config.Host) (*Profilinger, error) {
	var err error
	var s shell.Iface
	if host.IsLocal() {
		s = shell.NewLocalClient(exec.New())
	} else {
		s, err = shell.NewSshClient(host.Host, host.Port, host.User, host.Password, host.Key)
		if err != nil {
			return nil, err
		}
	}
	return &Profilinger{logger, s, templator}, nil
}

func (p Profilinger) Profiling(ctx context.Context, command string) error {
	command, err := p.template.Exec(command)
	if err != nil {
		return err
	}
	if _, stderr, err := p.shell.Exec(ctx, "", command); err != nil {
		return myerrros.NewErrorCommandExecutionFailed(stderr)
	}
	return nil
}
