package deploy

import (
	"go.uber.org/zap"

	"github.com/ShotaKitazawa/isu-continuous/pkg/config"
	"github.com/ShotaKitazawa/isu-continuous/pkg/shell"
	"github.com/ShotaKitazawa/isu-continuous/pkg/template"
)

type Deployer struct {
	log      *zap.Logger
	shell    shell.Iface
	template *template.Templator
}

func New(logger *zap.Logger, s shell.Iface, templator *template.Templator) *Deployer {
	return &Deployer{logger, s, templator}
}

func (d Deployer) Deploy(targets []config.DeployTarget) error {
}

func (d Deployer) RunCommand(command string) error {
}
