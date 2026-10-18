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
	SshKey       string
}

func RunInit(conf ConfigInit) (*InitResult, error) {
	return RunInitContext(context.Background(), conf)
}

func RunInitContext(ctx context.Context, conf ConfigInit) (*InitResult, error) {
	logger, err := newLogger(conf.LogLevel, conf.LogFilename)
	if err != nil {
		return nil, err
	}
	return runInit(conf, ctx, logger)
}

func runInit(conf ConfigInit, ctx context.Context, logger *zap.Logger) (*InitResult, error) {
	logger.Info("start init")
	defer func() { logger.Info("finish init") }()
	// Create local-repo directory if does not existed
	if _, err := os.Stat(conf.LocalRepoPath); err == nil {
		return nil, myerrors.NewErrorFileAlreadyExisted(conf.LocalRepoPath)
	}
	if err := os.Mkdir(filepath.Clean(conf.LocalRepoPath), 0755); err != nil {
		return nil, err
	}
	// Initialize local-repo
	_, warnings, err := localrepo.InitLocalRepoContext(ctx, logger, exec.New(), conf.LocalRepoPath, conf.GitUsername, conf.GitEmail, conf.GitRemoteUrl, conf.SshKey)
	if err != nil {
		return nil, err
	}
	return &InitResult{Result{Warnings: warnings}}, nil
}
