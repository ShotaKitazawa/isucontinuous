package cmd

import (
	"context"
	"os"
	"path/filepath"

	"github.com/ShotaKitazawa/isucontinuous/pkg/config"
	"github.com/ShotaKitazawa/isucontinuous/pkg/localrepo"
	"go.uber.org/zap"
	"k8s.io/utils/exec"
)

type ConfigImport struct {
	ConfigCommon
}

func RunImport(conf ConfigImport) error {
	ctx := context.Background()
	logger, err := newLogger(conf.LogLevel, conf.LogFilename)
	if err != nil {
		return err
	}
	return runImport(conf, ctx, logger)
}

func runImport(conf ConfigImport, ctx context.Context, logger *zap.Logger) error {
	// Attach local isucon-repo
	repo, err := localrepo.AttachLocalRepo(logger, exec.New(), conf.LocalRepoPath)
	if err != nil {
		return err
	}
	// Load isucontinuous.yaml
	isucontinuous, err := config.Load(conf.LocalRepoPath, isucontinuousFilename)
	if err != nil {
		return err
	}
	// List TargetDirs
	hosts := isucontinuous.ListTargetHosts()
	if err != nil {
		return err
	}
	// Import files from per host
	err = perHostExec(logger, ctx, hosts, func(ctx context.Context, host config.Host) error {
		// Import files
		for _, target := range host.ListTarget() {
			// if target is not file, notice to INFO and skip

			// if target is file, copy target

			// if target is directory, recursive copy excluded content of gitignore
			targetR, err := localrepo.InitLocalRepo(logger, exec.New(), target.Target, "dummy", "dummy", "dummy")
			if err != nil {
				return err
			}
			defer targetR.Clear()
			files, err := targetR.ListUntrackedFiles(ctx)
			for _, file := range files {
				absPath := filepath.Join(target.Target, file)
				fStat, err := os.Stat(absPath)
				if err != nil {
					return err
				}
				f, err := os.ReadFile(absPath)
				if err != nil {
					return err
				}
				repo.CreateFile(filepath.Join(host.Host, target.Src), f, fStat.Mode())
			}
		}
		return nil
	})

	return nil
}
