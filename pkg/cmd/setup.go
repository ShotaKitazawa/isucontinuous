package cmd

import (
	"context"

	"go.uber.org/zap"
	"k8s.io/utils/exec"

	"github.com/ShotaKitazawa/isu-continuous/pkg/config"
	"github.com/ShotaKitazawa/isu-continuous/pkg/localrepo"
	"github.com/ShotaKitazawa/isu-continuous/pkg/shell"
	"github.com/ShotaKitazawa/isu-continuous/pkg/usecases/install"
)

type ConfigSetup struct {
	ConfigCommon
}

func RunSetup(conf ConfigSetup) error {
	ctx := context.Background()
	logger, err := newLogger(conf.LogLevel, conf.LogFilename)
	if err != nil {
		return err
	}
	// Attach local isucon-repo
	repo, err := localrepo.AttachLocalRepo(logger, exec.New(), conf.LocalRepoPath)
	if err != nil {
		return err
	}
	// load isucontinuous.yaml
	isucontinuous, err := repo.LoadConf(isucontinuousFilename)
	if err != nil {
		return err
	}
	// set installers
	installers := make(map[string]*install.Installer)
	for _, host := range isucontinuous.Hosts {
		var s shell.Iface
		if host.IsLocal() {
			s = shell.NewLocalClient(exec.New())
		} else {
			s, err = shell.NewSshClient(host.Host, host.Port, host.User, host.Password, host.Key)
			if err != nil {
				return err
			}
		}
		installers[host.Host] = install.NewInstaller(logger, s)
	}
	return runSetup(conf, ctx, logger, repo, installers)
}

func runSetup(
	conf ConfigSetup, ctx context.Context, logger *zap.Logger,
	repo localrepo.LocalRepoIface, installers map[string]*install.Installer,
) error {
	// load isucontinuous.yaml
	isucontinuous, err := repo.LoadConf(isucontinuousFilename)
	if err != nil {
		return err
	}
	return perHostExec(logger, ctx, isucontinuous.Hosts, func(ctx context.Context, host config.Host) error {
		installer := installers[host.Host]
		// install docker
		if isucontinuous.IsDockerEnabled() {
			if err := installer.Docker(ctx); err != nil {
				return err
			}
		}
		// install netdata
		if ok, version, publicPort := isucontinuous.IsNetdataEnabled(); isucontinuous.IsDockerEnabled() && ok {
			if err := installer.Netdata(ctx, version, publicPort); err != nil {
				return err
			}
		}
		// install alp
		if ok, version := isucontinuous.IsAlpEnabled(); ok {
			if err := installer.Alp(ctx, version); err != nil {
				return err
			}
		}
		return nil
	})
}
