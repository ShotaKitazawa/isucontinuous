package cmd

import (
	"context"
	"os"
	"path/filepath"

	"go.uber.org/zap"
	"k8s.io/utils/exec"

	myerrors "github.com/ShotaKitazawa/isucontinuous/pkg/errors"
	"github.com/ShotaKitazawa/isucontinuous/pkg/localrepo"
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
	logger.Info("start init")
	defer func() { logger.Info("finish init") }()
	// Create local-repo directory if does not existed
	if _, err := os.Stat(conf.LocalRepoPath); err == nil {
		return myerrors.NewErrorFileAlreadyExisted(conf.LocalRepoPath)
	}
	if err := os.Mkdir(filepath.Clean(conf.LocalRepoPath), 0755); err != nil {
		return err
	}
	// Initialize local-repo
	_, err := localrepo.InitLocalRepo(logger, exec.New(), conf.LocalRepoPath, conf.GitUsername, conf.GitEmail, conf.GitRemoteUrl)
	if err != nil {
		return err
	}
	return nil
}
