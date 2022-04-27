package deploy

import (
	"context"
	"fmt"
	"io/fs"
	"path/filepath"
	"reflect"
	"strings"

	"go.uber.org/zap"

	"github.com/ShotaKitazawa/isu-continuous/pkg/config"
	myerrros "github.com/ShotaKitazawa/isu-continuous/pkg/errors"
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

func (d Deployer) Deploy(ctx context.Context, targets []config.DeployTarget) error {
	for _, target := range targets {
		if err := filepath.WalkDir(target.Src, func(path string, info fs.DirEntry, err error) error {
			if info != nil && reflect.ValueOf(info).IsNil() && !info.IsDir() {
				dst := filepath.Join(target.Target, strings.TrimLeft(path, target.Src))
				d.log.Debug(fmt.Sprintf("deploy %s to %s", path, dst), zap.String("host", d.shell.Host()))
				return d.shell.Deploy(ctx, path, dst)
			}
			return nil
		}); err != nil {
			return err
		}
		if target.Compile != "" {
			d.log.Debug(fmt.Sprintf(`exec compile: "%s"`, target.Compile), zap.String("host", d.shell.Host()))
			if _, stderr, err := d.shell.Exec(ctx, target.Target, target.Compile); err != nil {
				return myerrros.NewErrorCommandExecutionFailed(stderr)
			}
		}
	}
	return nil
}

func (d Deployer) RunCommand(ctx context.Context, command string) error {
	var err error
	command, err = d.template.Exec(command)
	if err != nil {
		return err
	}
	d.log.Debug(fmt.Sprintf(`exec command: "%s"`, command), zap.String("host", d.shell.Host()))
	_, stderr, err := d.shell.Exec(ctx, "", command)
	if err != nil {
		return myerrros.NewErrorCommandExecutionFailed(stderr)
	}
	return nil
}
