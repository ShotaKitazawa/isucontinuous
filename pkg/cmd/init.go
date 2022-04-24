package cmd

import (
	"context"
	"os"

	"go.uber.org/zap"
	"k8s.io/utils/exec"

	"github.com/ShotaKitazawa/isu-continuous/pkg/config"
	myerrors "github.com/ShotaKitazawa/isu-continuous/pkg/errors"
	"github.com/ShotaKitazawa/isu-continuous/pkg/localrepo"
)

type ConfigInit struct {
	ConfigCommon
	GitUsername  string
	GitEmail     string
	GitRemoteUrl string
}

func RunInit(conf ConfigInit) error {
	ctx := context.Background()
	logger, err := newLogger(conf.LogLevel, conf.LogFilename)
	if err != nil {
		return err
	}
	return runInit(conf, ctx, logger)
}

func runInit(conf ConfigInit, ctx context.Context, logger *zap.Logger) error {
	// Create local-repo directory if does not existed
	if _, err := os.Stat(conf.LocalRepoPath); err == nil {
		return myerrors.NewErrorFileAlreadyExisted(conf.LocalRepoPath)
	}
	if err := os.Mkdir(conf.LocalRepoPath, 0755); err != nil {
		return err
	}
	// Initialize local-repo
	repo, err := localrepo.InitLocalRepo(logger, exec.New(), conf.LocalRepoPath, conf.GitUsername, conf.GitEmail, conf.GitRemoteUrl)
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
