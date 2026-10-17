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
	if host.Local {
		s = shell.NewLocalClient(exec.New())
	} else {
		s, err = shell.NewSshClient(host.Host, host.Port, host.User, host.Password, host.Key)
		if err != nil {
			return nil, err
		}
	}
	return NewWithShell(logger, s, templator, slackClient), nil
}

// NewWithShell builds an AfterBencher on an existing shell, which lets callers substitute a fake host.
func NewWithShell(logger *zap.Logger, s shell.Iface, templator *template.Templator, slackClient slack.ClientIface) *AfterBencher {
	return &AfterBencher{logger, s, templator, slackClient}
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

// Artifact is a file produced by the commands, read once for both Slack and the result.
type Artifact struct {
	Path    string
	Content string
}

// PostToSlack posts every file under dir and returns them; on failure it returns those read so far.
func (p AfterBencher) PostToSlack(ctx context.Context, dir, channel string) ([]Artifact, error) {
	dir, err := p.template.Exec(dir)
	dir = filepath.Clean(dir)
	if err != nil {
		return nil, err
	}
	stdout, stderr, err := p.shell.Execf(ctx, "", `find "%s" -type f`, dir)
	if err != nil {
		return nil, myerrros.NewErrorCommandExecutionFailed(stderr)
	}
	var artifacts []Artifact
	for _, filename := range strings.Split(stdout.String(), "\n") {
		if filename == "" {
			continue
		}
		stdout, stderr, err := p.shell.Execf(ctx, "", "cat %s", filename)
		if err != nil {
			return artifacts, myerrros.NewErrorCommandExecutionFailed(stderr)
		}
		artifacts = append(artifacts, Artifact{Path: filename, Content: stdout.String()})
		title := fmt.Sprintf("%s at %s (%s)", filepath.Base(filename), p.shell.Host(), p.template.Git.Revision)
		if err := p.slack.SendFileContent(ctx, channel, filename, stdout.String(), title); err != nil {
			return artifacts, err
		}
	}
	return artifacts, nil
}

func (p AfterBencher) Prepare(ctx context.Context, dir string) error {
	dir, err := p.template.Exec(dir)
	dir = filepath.Clean(dir)
	if err != nil {
		return err
	}
	if _, stderr, err := p.shell.Execf(ctx, "", `mkdir -p "%s"`, dir); err != nil {
		return myerrros.NewErrorCommandExecutionFailed(stderr)
	}
	return nil
}

// Moved is where the output directory went at the end of the run.
type Moved struct {
	From, To string
}

// Path converts a path under From to the same path under To.
func (m Moved) Path(p string) string {
	if m.From != "" && (p == m.From || strings.HasPrefix(p, m.From+"/")) {
		return m.To + strings.TrimPrefix(p, m.From)
	}
	return p
}

func (p AfterBencher) CleanUp(ctx context.Context, dir, suffix string) (Moved, error) {
	srcDir, err := p.template.Exec(dir)
	srcDir = filepath.Clean(srcDir)
	if err != nil {
		return Moved{}, err
	}
	dstDir := fmt.Sprintf("%s.%s", srcDir, suffix)
	if _, stderr, err := p.shell.Execf(ctx, "", `mv "%s" "%s"`, srcDir, dstDir); err != nil {
		return Moved{}, myerrros.NewErrorCommandExecutionFailed(stderr)
	}
	return Moved{From: srcDir, To: dstDir}, nil
}
