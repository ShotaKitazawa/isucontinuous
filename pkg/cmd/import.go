package cmd

import (
	"context"
	"fmt"
	"path/filepath"

	"go.uber.org/zap"
	"k8s.io/utils/exec"

	"github.com/ShotaKitazawa/isucontinuous/pkg/config"
	myerrors "github.com/ShotaKitazawa/isucontinuous/pkg/errors"
	"github.com/ShotaKitazawa/isucontinuous/pkg/localrepo"
	"github.com/ShotaKitazawa/isucontinuous/pkg/usecases/imports"
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
	// Attach local isucon-repo
	repo, err := localrepo.AttachLocalRepo(logger, exec.New(), conf.LocalRepoPath)
	if err != nil {
		return err
	}
	return runImport(conf, ctx, logger, repo, imports.NewImporters)
}

func runImport(
	conf ConfigImport, ctx context.Context, logger *zap.Logger,
	repo localrepo.LocalRepoIface, newImporters imports.NewImportersFunc,
) error {
	logger.Info("start import")
	defer func() { logger.Info("finish import") }()
	// Check currentBranch
	if _, err := repo.CurrentBranch(ctx); err != nil {
		return err
	}
	// load isucontinuous.yaml
	isucontinuous, err := repo.LoadConf()
	if err != nil {
		return err
	}
	// Set importers
	importers, err := newImporters(logger, isucontinuous.Hosts)
	if err != nil {
		return err
	}
	// Import files from per host
	return perHostExec(logger, ctx, isucontinuous.Hosts, []task{{
		"Import",
		func(ctx context.Context, host config.Host) error {
			importer := importers[host.Host]
			for _, target := range host.ListTarget() {
				switch importer.FileType(ctx, target.Target) {
				case imports.IsNotFound:
					logger.Info(fmt.Sprintf("%s is not found: skip", target.Target), zap.String("host", host.Host))
					continue
				case imports.IsFile:
					content, mode, err := importer.GetFileContent(ctx, target.Target)
					if err != nil {
						return err
					}
					if err := repo.CreateFile(filepath.Join(host.Host, target.Src), content, mode); err != nil {
						return err
					}
				case imports.IsDirectory:
					files, err := importer.ListUntrackedFiles(ctx, target.Target)
					if err != nil {
						return err
					}
					files = importer.ExcludeSymlinkFiles(ctx, files)
					for _, file := range files {
						fileAbsPath := filepath.Join(target.Target, file)
						content, mode, err := importer.GetFileContent(ctx, fileAbsPath)
						if err != nil {
							return err
						}
						if err := repo.CreateFile(filepath.Join(host.Host, target.Src, file), content, mode); err != nil {
							return err
						}
					}
				default:
					return myerrors.NewErrorUnkouwn()
				}
			}
			return nil
		},
	}})
}
