package cmd

import (
	"k8s.io/utils/exec"

	"github.com/ShotaKitazawa/isucontinuous/pkg/config"
	"github.com/ShotaKitazawa/isucontinuous/pkg/gitcommand"
)

type ConfigInit struct {
	ConfigCommon
	GitUsername  string
	GitEmail     string
	GitRemoteUrl string
}

func RunInit(conf ConfigInit) error {
	logger, err := newLogger(conf.LogFilename)
	if err != nil {
		return err
	}

	// Create local-repo if does not existed
	repo, err := gitcommand.NewLocalRepo(logger, exec.New(), conf.LocalRepoPath, conf.GitUsername, conf.GitEmail, conf.GitRemoteUrl)
	if err != nil {
		return err
	}

	// Generate skelton
	f, err := config.SkeltonBytes()
	if err != nil {
		return err
	}

	// Create isucontinuous.yaml to local-repo.
	if err := repo.CreateFile(isucontinuousFilename, f, 0644); err != nil {
		return err
	}

	return nil
}
