package cmd

import (
	"context"
	"fmt"
	"path/filepath"

	"go.uber.org/zap"
	"k8s.io/utils/exec"

	"github.com/ShotaKitazawa/isucontinuous/pkg/config"
	myerrors "github.com/ShotaKitazawa/isucontinuous/pkg/errors"
	"github.com/ShotaKitazawa/isucontinuous/pkg/imports"
	"github.com/ShotaKitazawa/isucontinuous/pkg/localrepo"
	"github.com/ShotaKitazawa/isucontinuous/pkg/shell"
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
	// load isucontinuous.yaml
	isucontinuous, err := config.Load(conf.LocalRepoPath, isucontinuousFilename)
	if err != nil {
		return err
	}
	// set importers
	importers := make(map[string]*imports.Importer)
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
		importers[host.Host] = imports.New(logger, s)
	}
	return runImport(conf, ctx, logger, isucontinuous, importers)
}

func runImport(
	conf ConfigImport, ctx context.Context, logger *zap.Logger,
	isucontinuous *config.Config, importers map[string]*imports.Importer,
) error {
	// Attach local isucon-repo
	repo, err := localrepo.AttachLocalRepo(logger, exec.New(), conf.LocalRepoPath)
	if err != nil {
		return err
	}
	// List TargetDirs
	hosts := isucontinuous.ListTargetHosts()
	if err != nil {
		return err
	}
	// Import files from per host
	return perHostExec(logger, ctx, hosts, func(ctx context.Context, host config.Host) error {
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
				for _, file := range files {
					fileAbsPath := filepath.Join(target.Target, file)
					content, mode, err := importer.GetFileContent(ctx, fileAbsPath)
					if err != nil {
						return err
					}
					if err := repo.CreateFile(filepath.Join(host.Host, target.Src), content, mode); err != nil {
						return err
					}
				}
			default:
				return myerrors.NewErrorUnkouwn()
			}
		}
		return nil
	})
}
