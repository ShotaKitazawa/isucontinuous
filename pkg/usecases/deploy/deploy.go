package deploy

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"go.uber.org/zap"
	"k8s.io/utils/exec"

	"github.com/ShotaKitazawa/isucontinuous/pkg/config"
	myerrros "github.com/ShotaKitazawa/isucontinuous/pkg/errors"
	"github.com/ShotaKitazawa/isucontinuous/pkg/shell"
	"github.com/ShotaKitazawa/isucontinuous/pkg/template"
)

type Deployer struct {
	log           *zap.Logger
	shell         shell.Iface
	template      *template.Templator
	localRepoPath string
}

type NewDeployersFunc func(*zap.Logger, *template.Templator, string, []config.Host) (map[string]*Deployer, error)

func NewDeployers(
	logger *zap.Logger, templator *template.Templator, localRepoPath string,
	hosts []config.Host,
) (map[string]*Deployer, error) {
	deployers := make(map[string]*Deployer)
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
		deployers[host.Host] = new(logger, s, templator, localRepoPath)
	}
	return deployers, nil
}

func new(logger *zap.Logger, s shell.Iface, templator *template.Templator, localRepoPath string) *Deployer {
	return &Deployer{logger, s, templator, localRepoPath}
}

func (d Deployer) Deploy(ctx context.Context, targets []config.DeployTarget) error {
	host := d.shell.Host()
	for _, target := range targets {
		src := filepath.Join(d.localRepoPath, host, target.Src)
		if err := filepath.WalkDir(src, func(path string, info fs.DirEntry, err error) error {
			if info != nil && !reflect.ValueOf(info).IsNil() && !info.IsDir() {
				dst := filepath.Join(target.Target, strings.TrimPrefix(path, src))
				dirname := filepath.Dir(dst)
				if _, _, err := d.shell.Execf(ctx, "", `test -d "%s"`, dirname); err != nil {
					d.log.Debug(fmt.Sprintf("%s does not exist, mkdir", dirname), zap.String("host", host))
					if _, _, err := d.shell.Execf(ctx, "", `mkdir -p "%s"`, dirname); err != nil {
						return err
					}
				}
				finfo, err := info.Info()
				if err != nil {
					return err
				}
				if finfo.Mode()&os.ModeSymlink == os.ModeSymlink { // copy source is symlink
					origin, err := filepath.EvalSymlinks(path)
					if err != nil {
						return err
					}
					originAbs, err := filepath.Abs(origin)
					if err != nil {
						return err
					}
					newSrc := strings.TrimPrefix(originAbs, filepath.Join(d.localRepoPath, host)+"/")
					if newSrc == originAbs {
						return fmt.Errorf("%s: cannot seek symlink", originAbs)
					}
					d.log.Debug(fmt.Sprintf("%s is symlink, seek to %s", path, originAbs), zap.String("host", host))
					if err := d.Deploy(ctx, []config.DeployTarget{{Src: newSrc, Target: dst}}); err != nil {
						return err
					}
				} else { // copy source is file
					d.log.Debug(fmt.Sprintf("deploy %s to %s", path, dst), zap.String("host", host))
					return d.shell.Deploy(ctx, path, dst)
				}
			}
			return nil
		}); err != nil {
			return err
		}
		if target.Compile != "" {
			d.log.Debug(fmt.Sprintf(`exec compile: "%s"`, target.Compile), zap.String("host", host))
			if _, stderr, err := d.shell.Exec(ctx, target.Target, target.Compile); err != nil {
				return myerrros.NewErrorCommandExecutionFailed(stderr)
			}
		}
	}
	return nil
}

func (d Deployer) RunCommand(ctx context.Context, command string) error {
	if command == "" {
		return nil
	}
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
