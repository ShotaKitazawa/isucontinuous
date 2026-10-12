package deploy

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"go.uber.org/zap"
	"k8s.io/utils/exec"

	"github.com/ShotaKitazawa/isucontinuous/pkg/config"
	myerrros "github.com/ShotaKitazawa/isucontinuous/pkg/errors"
	"github.com/ShotaKitazawa/isucontinuous/pkg/hoststate"
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
		if host.Local {
			s = shell.NewLocalClient(exec.New())
		} else {
			s, err = shell.NewSshClient(host.Host, host.Port, host.User, host.Password, host.Key)
			if err != nil {
				return nil, err
			}
		}
		deployers[host.Name] = new(logger, s, templator, localRepoPath)
	}
	return deployers, nil
}

func new(logger *zap.Logger, s shell.Iface, templator *template.Templator, localRepoPath string) *Deployer {
	return &Deployer{logger, s, templator, localRepoPath}
}

// PlanFile maps a file in local-repo to its destination on the host.
type PlanFile struct {
	Src      string
	Dst      string
	RepoPath string // slash-separated path from the root of local-repo, as used by `git show`
}

type TargetPlan struct {
	Files      []PlanFile
	Compile    string
	CompileDir string
}

// Plan is what Deploy would do; it is computed without touching the host.
type Plan struct {
	Targets []TargetPlan
}

func (p *Plan) files() []PlanFile {
	var files []PlanFile
	for _, t := range p.Targets {
		files = append(files, t.Files...)
	}
	return files
}

func (d Deployer) Deploy(ctx context.Context, host string, targets []config.DeployTarget) error {
	plan, err := d.Plan(ctx, host, targets)
	if err != nil {
		return err
	}
	return d.Execute(ctx, host, plan)
}

// Plan maps files of host in local-repo to their destinations, following symlinks.
func (d Deployer) Plan(ctx context.Context, host string, targets []config.DeployTarget) (*Plan, error) {
	plan := &Plan{}
	for _, target := range targets {
		files, err := d.planFiles(ctx, host, target.Src, target.Target)
		if err != nil {
			return nil, err
		}
		plan.Targets = append(plan.Targets, TargetPlan{Files: files, Compile: target.Compile, CompileDir: target.Target})
	}
	return plan, nil
}

func (d Deployer) planFiles(ctx context.Context, host, srcRel, dstRoot string) ([]PlanFile, error) {
	realHost := d.shell.Host()
	src := filepath.Join(d.localRepoPath, host, srcRel)
	var files []PlanFile
	err := filepath.WalkDir(src, func(path string, info fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			return nil
		}
		dst := filepath.Join(dstRoot, strings.TrimPrefix(path, src))
		finfo, err := info.Info()
		if err != nil {
			return err
		}
		if finfo.Mode()&os.ModeSymlink == os.ModeSymlink { // copy source is symlink
			origin, err := filepath.EvalSymlinks(path)
			if err != nil {
				return err
			}
			newHostAndSrc := strings.TrimPrefix(origin, d.localRepoPath+"/")
			if newHostAndSrc == origin {
				return fmt.Errorf("%s: cannot seek symlink bacause source of symlic isn't in localRepoPath", origin)
			}
			newHostAndSrcSlice := strings.Split(newHostAndSrc, "/")
			newHost := newHostAndSrcSlice[0]
			newSrc := strings.Join(newHostAndSrcSlice[1:], "/")
			if len(newHostAndSrcSlice) < 2 {
				return fmt.Errorf("%s: cannot seek symlink bacause this directory is hostdir", origin)
			}
			d.log.Debug(fmt.Sprintf("%s is symlink, seek to %s", path, origin), zap.String("host", realHost))
			sub, err := d.planFiles(ctx, newHost, newSrc, dst)
			if err != nil {
				return err
			}
			files = append(files, sub...)
			return nil
		}
		rel, err := filepath.Rel(d.localRepoPath, path)
		if err != nil {
			return err
		}
		files = append(files, PlanFile{Src: path, Dst: dst, RepoPath: filepath.ToSlash(rel)})
		return nil
	})
	return files, err
}

// Execute transfers only the files whose content differs on the host, then runs compile commands.
func (d Deployer) Execute(ctx context.Context, host string, plan *Plan) error {
	realHost := d.shell.Host()
	for _, target := range plan.Targets {
		changed, err := d.changedFiles(ctx, target.Files)
		if err != nil {
			return err
		}
		if err := d.mkdirAll(ctx, changed); err != nil {
			return err
		}
		for _, f := range changed {
			d.log.Debug(fmt.Sprintf("deploy %s to %s", f.Src, f.Dst), zap.String("host", realHost))
			if err := d.shell.Deploy(ctx, f.Src, f.Dst); err != nil {
				return err
			}
		}
		if target.Compile != "" {
			d.log.Debug(fmt.Sprintf(`exec compile: "%s"`, target.Compile), zap.String("host", host))
			if _, stderr, err := d.shell.Exec(ctx, target.CompileDir, target.Compile); err != nil {
				return myerrros.NewErrorCommandExecutionFailed(stderr)
			}
		}
	}
	return nil
}

func (d Deployer) mkdirAll(ctx context.Context, files []PlanFile) error {
	seen := make(map[string]bool)
	var dirs []string
	for _, f := range files {
		dir := filepath.Dir(f.Dst)
		if !seen[dir] {
			seen[dir] = true
			dirs = append(dirs, shell.Quote(dir))
		}
	}
	if len(dirs) == 0 {
		return nil
	}
	_, stderr, err := d.shell.Execf(ctx, "", "mkdir -p %s", strings.Join(dirs, " "))
	if err != nil {
		return myerrros.NewErrorCommandExecutionFailed(stderr)
	}
	return nil
}

func (d Deployer) RecordRevision(ctx context.Context, hash string) error {
	return hoststate.WriteLastRevision(ctx, d.shell, hash)
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
