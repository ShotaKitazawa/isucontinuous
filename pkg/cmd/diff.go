package cmd

import (
	"context"

	"go.uber.org/zap"
	"k8s.io/utils/exec"

	"github.com/ShotaKitazawa/isucontinuous/pkg/config"
	"github.com/ShotaKitazawa/isucontinuous/pkg/localrepo"
	"github.com/ShotaKitazawa/isucontinuous/pkg/shell"
	"github.com/ShotaKitazawa/isucontinuous/pkg/template"
	"github.com/ShotaKitazawa/isucontinuous/pkg/usecases/deploy"
)

type ConfigDiff struct {
	ConfigCommon
	// GitRevision empty means the revision each host was deployed with last
	GitRevision string
	ShowDiff    bool
}

func RunDiff(conf ConfigDiff) (*DiffResult, error) {
	ctx := context.Background()
	logger, err := newLogger(conf.LogLevel, conf.LogFilename)
	if err != nil {
		return nil, err
	}
	shell.SetKnownHostsPath(conf.KnownHostsPath)
	// Attach local-repo
	repo, err := localrepo.AttachLocalRepo(logger, exec.New(), conf.LocalRepoPath)
	if err != nil {
		return nil, err
	}
	return runDiff(conf, ctx, logger, repo, deploy.NewDeployers)
}

func runDiff(
	conf ConfigDiff, ctx context.Context, logger *zap.Logger,
	repo localrepo.LocalRepoIface, newDeployersFunc deploy.NewDeployersFunc,
) (*DiffResult, error) {
	logger.Info("start diff")
	defer func() { logger.Info("finish diff") }()
	// The working tree is left as it is; only the revision to compare is looked up.
	var hash string
	if conf.GitRevision != "" {
		if err := repo.Fetch(ctx); err != nil {
			return nil, err
		}
		var err error
		if hash, err = repo.ResolveRevision(ctx, conf.GitRevision); err != nil {
			return nil, err
		}
	}
	// Load isucontinus.yaml
	isucontinuous, err := repo.LoadConf()
	if err != nil {
		return nil, err
	}
	result := &DiffResult{}
	result.Warnings = isucontinuous.Warnings
	deployers, err := newDeployersFunc(logger, template.New(hash), conf.LocalRepoPath, isucontinuous.Hosts)
	if err != nil {
		return result, err
	}
	diffs := make([]deploy.HostDiff, len(isucontinuous.Hosts))
	index := make(map[string]int, len(isucontinuous.Hosts))
	for idx, host := range isucontinuous.Hosts {
		index[host.Name] = idx
		diffs[idx] = deploy.HostDiff{Host: host.Name}
	}
	hosts, err := perHostExec(logger, ctx, conf.reporter(), isucontinuous.Hosts, []task{{
		"Diff",
		func(ctx context.Context, host config.Host) error {
			deployer := deployers[host.Name]
			target := hash
			if target == "" {
				last, found, err := deployer.LastRevision(ctx)
				if err != nil {
					return err
				}
				if !found {
					diffs[index[host.Name]].NoRecord = true
					return nil
				}
				target = last
			}
			d, err := deployer.Diff(ctx, host.Name, host.Deploy.Targets, target, repo, conf.ShowDiff)
			if err != nil {
				return err
			}
			diffs[index[host.Name]] = *d
			return nil
		},
	}})
	result.Hosts = hosts
	result.Diffs = diffs
	return result, err
}
