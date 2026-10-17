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
	"github.com/ShotaKitazawa/isucontinuous/pkg/shell"
	"github.com/ShotaKitazawa/isucontinuous/pkg/usecases/imports"
	"github.com/ShotaKitazawa/isucontinuous/pkg/usecases/lock"
)

type ConfigImport struct {
	ConfigCommon
}

func RunImport(conf ConfigImport) (*ImportResult, error) {
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
	return runImport(conf, ctx, logger, repo, imports.NewImporters, lock.NewLockers)
}

func runImport(
	conf ConfigImport, ctx context.Context, logger *zap.Logger,
	repo localrepo.LocalRepoIface, newImporters imports.NewImportersFunc, newLockersFunc lock.NewLockersFunc,
) (*ImportResult, error) {
	logger.Info("start import")
	defer func() { logger.Info("finish import") }()
	// Check currentBranch
	if _, err := repo.CurrentBranch(ctx); err != nil {
		return nil, err
	}
	// load isucontinuous.yaml
	isucontinuous, err := repo.LoadConf()
	if err != nil {
		return nil, err
	}
	// Importing while someone else experiments would take in a half-changed state
	unlock, err := lockForCommand(ctx, logger, conf.ConfigCommon, repo, newLockersFunc, isucontinuous.Hosts, "import")
	if err != nil {
		return &ImportResult{Result{Warnings: isucontinuous.Warnings}}, err
	}
	defer unlock()
	// Set importers
	importers, err := newImporters(logger, isucontinuous.Hosts)
	if err != nil {
		return nil, err
	}
	// Import files from per host
	hosts, err := perHostExec(logger, ctx, conf.reporter(), isucontinuous.Hosts, []task{{
		"Import",
		func(ctx context.Context, host config.Host) error {
			importer := importers[host.Name]
			for _, target := range host.ListTarget() {
				switch importer.FileType(ctx, target.Target) {
				case imports.IsNotFound:
					logger.Info(fmt.Sprintf("%s is not found: skip", target.Target), zap.String("host", host.Name))
					continue
				case imports.IsFile:
					content, mode, err := importer.GetFileContent(ctx, target.Target)
					if err != nil {
						return err
					}
					if err := repo.CreateFile(filepath.Join(host.Name, target.Src), content, mode); err != nil {
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
						if err := repo.CreateFile(filepath.Join(host.Name, target.Src, file), content, mode); err != nil {
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
	return &ImportResult{Result{Hosts: hosts, Warnings: isucontinuous.Warnings}}, err
}
