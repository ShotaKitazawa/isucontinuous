package afterbench

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"go.uber.org/zap"
	"k8s.io/utils/exec"

	"github.com/ShotaKitazawa/isucontinuous/pkg/config"
	myerrros "github.com/ShotaKitazawa/isucontinuous/pkg/errors"
	"github.com/ShotaKitazawa/isucontinuous/pkg/shell"
	"github.com/ShotaKitazawa/isucontinuous/pkg/slack"
	"github.com/ShotaKitazawa/isucontinuous/pkg/template"
)

type AfterBencher struct {
	log      *zap.Logger
	shell    shell.Iface
	template *template.Templator
	slack    slack.ClientIface
}

type NewFunc func(*zap.Logger, *template.Templator, slack.ClientIface, config.Host) (*AfterBencher, error)

func New(logger *zap.Logger, templator *template.Templator, slackClient slack.ClientIface, host config.Host) (*AfterBencher, error) {
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
	return &AfterBencher{logger, s, templator, slackClient}, nil
}

func (p AfterBencher) RunCommand(ctx context.Context, command string) error {
	command, err := p.template.Exec(command)
	if err != nil {
		return err
	}
	if _, stderr, err := p.shell.Exec(ctx, "", command); err != nil {
		return myerrros.NewErrorCommandExecutionFailed(stderr)
	}
	return nil
}

func (p AfterBencher) PostToSlack(ctx context.Context, dir, channel string) error {
	dir, err := p.template.Exec(dir)
	if err != nil {
		return err
	}
	stdout, stderr, err := p.shell.Execf(ctx, "", "find %s -type f", dir)
	if err != nil {
		return myerrros.NewErrorCommandExecutionFailed(stderr)
	}
	for _, filename := range strings.Split(stdout.String(), "\n") {
		stdout, stderr, err := p.shell.Execf(ctx, "", "cat %s", filename)
		if err != nil {
			return myerrros.NewErrorCommandExecutionFailed(stderr)
		}
		title := fmt.Sprintf("%s at %s (%s)", filepath.Base(filename), p.shell.Host(), p.template.Git.Revision)
		if err := p.slack.SendFileContent(ctx, channel, filename, stdout.String(), title); err != nil {
			return err
		}
	}
	return nil
}

func (p AfterBencher) CleanUp(ctx context.Context, dir, suffix string) error {
	srcDir, err := p.template.Exec(dir)
	if err != nil {
		return err
	}
	dstDir := fmt.Sprintf("%s.%s", srcDir, suffix)
	if _, stderr, err := p.shell.Execf(ctx, "", "mv %s %s", srcDir, dstDir); err != nil {
		return myerrros.NewErrorCommandExecutionFailed(stderr)
	}
	return nil
}
