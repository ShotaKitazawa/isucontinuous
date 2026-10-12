package cmd

import (
	"context"

	"go.uber.org/zap"
	"k8s.io/utils/exec"

	"github.com/ShotaKitazawa/isucontinuous/pkg/config"
	"github.com/ShotaKitazawa/isucontinuous/pkg/localrepo"
	"github.com/ShotaKitazawa/isucontinuous/pkg/shell"
	"github.com/ShotaKitazawa/isucontinuous/pkg/usecases/install"
	"github.com/ShotaKitazawa/isucontinuous/pkg/usecases/lock"
)

type ConfigSetup struct {
	ConfigCommon
}

func RunSetup(conf ConfigSetup) (*SetupResult, error) {
	ctx := context.Background()
	logger, err := newLogger(conf.LogLevel, conf.LogFilename)
	if err != nil {
		return nil, err
	}
	shell.SetKnownHostsPath(conf.KnownHostsPath)
	// Attach local isucon-repo
	repo, err := localrepo.AttachLocalRepo(logger, exec.New(), conf.LocalRepoPath)
	if err != nil {
		return nil, err
	}
	// set installers
	return runSetup(conf, ctx, logger, repo, install.NewInstallers, lock.NewLockers)
}

func runSetup(
	conf ConfigSetup, ctx context.Context, logger *zap.Logger,
	repo localrepo.LocalRepoIface, newInstallers install.NewInstallersFunc, newLockersFunc lock.NewLockersFunc,
) (*SetupResult, error) {
	logger.Info("start setup")
	defer func() { logger.Info("finish setup") }()
	// load isucontinuous.yaml
	isucontinuous, err := repo.LoadConf()
	if err != nil {
		return nil, err
	}
	// Setup changes the host, which affects a measurement in progress
	unlock, err := lockForCommand(ctx, logger, conf.ConfigCommon, repo, newLockersFunc, isucontinuous.Hosts, "setup")
	if err != nil {
		return &SetupResult{Result{Warnings: isucontinuous.Warnings}}, err
	}
	defer unlock()
	// Set installers
	installers, err := newInstallers(logger, isucontinuous.Hosts)
	if err != nil {
		return nil, err
	}
	hosts, err := perHostExec(logger, ctx, conf.reporter(), isucontinuous.Hosts, []task{{
		"Install Docker",
		func(ctx context.Context, host config.Host) error {
			installer := installers[host.Name]
			// install docker
			if isucontinuous.IsDockerEnabled() {
				if err := installer.Docker(ctx); err != nil {
					return err
				}
			}
			return nil
		},
	}, {
		"Install netdata",
		func(ctx context.Context, host config.Host) error {
			installer := installers[host.Name]
			if ok, version, publicPort := isucontinuous.IsNetdataEnabled(); isucontinuous.IsDockerEnabled() && ok {
				if err := installer.Netdata(ctx, version, publicPort); err != nil {
					return err
				}
			}
			return nil
		},
	}, {
		"Install alp",
		func(ctx context.Context, host config.Host) error {
			installer := installers[host.Name]
			if ok, version := isucontinuous.IsAlpEnabled(); ok {
				if err := installer.Alp(ctx, version); err != nil {
					return err
				}
			}
			return nil
		},
	}})
	return &SetupResult{Result{Hosts: hosts, Warnings: isucontinuous.Warnings}}, err
}
