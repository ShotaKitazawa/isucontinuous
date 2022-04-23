package cmd

import (
	"context"
	_ "embed"

	"github.com/ShotaKitazawa/isucontinuous/pkg/config"
	"github.com/ShotaKitazawa/isucontinuous/pkg/install"
	"k8s.io/utils/exec"
)

type ConfigSetup struct {
	ConfigCommon
}

func RunSetup(conf ConfigSetup) error {
	ctx := context.Background()
	logger, err := newLogger(conf.LogFilename)
	if err != nil {
		return err
	}
	installer := install.NewInstaller(logger, exec.New())

	// load isucontinuous.yaml
	isucontinuous, err := config.Load(isucontinuousFilename)
	if err != nil {
		return err
	}

	// install docker
	if isucontinuous.IsDockerEnabled() {
		if err := installer.Docker(ctx); err != nil {
			return err
		}
		// install netdata
		if ok, version, publicPort := isucontinuous.IsNetdataEnabled(); ok {
			if err := installer.Netdata(ctx, version, publicPort); err != nil {
				return err
			}
		}
	}

	// install alp
	if ok, version := isucontinuous.IsAlpEnabled(); ok {
		if err := installer.Alp(ctx, version); err != nil {
			return err
		}
	}

	return nil
}
