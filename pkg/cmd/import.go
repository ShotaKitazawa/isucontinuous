package cmd

import (
	"context"
	"errors"
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
	GitBranch string
	Message   string
}

func RunImport(conf ConfigImport) (*ImportResult, error) {
	return RunImportContext(context.Background(), conf)
}

func RunImportContext(ctx context.Context, conf ConfigImport) (*ImportResult, error) {
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
	unlock, err := repo.Lock(ctx)
	if err != nil {
		return nil, err
	}
	defer unlock()
	return runImport(conf, ctx, logger, repo, imports.NewImporters, lock.NewLockers)
}

func runImport(
	conf ConfigImport, ctx context.Context, logger *zap.Logger,
	repo localrepo.LocalRepoIface, newImporters imports.NewImportersFunc, newLockersFunc lock.NewLockersFunc,
) (*ImportResult, error) {
	logger.Info("start import")
	defer func() { logger.Info("finish import") }()
	if conf.GitBranch == "" {
		conf.GitBranch = "master"
	}
	firstCommit, err := repo.IsFirstCommit(ctx)
	if err != nil {
		return nil, err
	}
	if firstCommit {
		currentBranch, err := repo.CurrentBranch(ctx)
		if err != nil {
			return nil, err
		}
		if currentBranch != conf.GitBranch {
			return nil, fmt.Errorf("current branch name is %s; initialize or select %s before the first import", currentBranch, conf.GitBranch)
		}
	} else {
		if err := syncForImport(ctx, repo, conf.GitBranch); err != nil {
			return nil, err
		}
	}

	result, err := runImportFiles(conf, ctx, logger, repo, newImporters, newLockersFunc)
	if err != nil {
		return result, err
	}
	pushResult, err := pushForImport(ctx, repo, conf.GitBranch, conf.Message)
	if err != nil {
		return result, err
	}
	result.Files = pushResult.Files
	for _, warning := range pushResult.Warnings {
		result.Warnings = appendUnique(result.Warnings, warning)
	}
	return result, nil
}

func runImportFiles(
	conf ConfigImport, ctx context.Context, logger *zap.Logger,
	repo localrepo.LocalRepoIface, newImporters imports.NewImportersFunc, newLockersFunc lock.NewLockersFunc,
) (*ImportResult, error) {
	// load isucontinuous.yaml
	isucontinuous, err := repo.LoadConf()
	if err != nil {
		return nil, err
	}
	// Importing while someone else experiments would take in a half-changed state
	unlock, err := lockForCommand(ctx, logger, conf.ConfigCommon, repo, newLockersFunc, isucontinuous.Hosts, "import")
	if err != nil {
		return &ImportResult{Result: Result{Warnings: isucontinuous.Warnings}}, err
	}
	defer unlock()
	warnings := &warningList{}
	for _, warning := range isucontinuous.Warnings {
		warnings.AddUnique(warning)
	}
	// Set importers
	importers, err := newImporters(logger, isucontinuous.Hosts)
	if err != nil {
		return nil, err
	}
	// Import files from per host
	hosts, err := perHostExec(logger, ctx, conf.reporter(), isucontinuous.Hosts, []task{{
		"Import",
		func(ctx context.Context, host config.Host) error {
			skip, err := checkImportDestination(repo, warnings, host.Name, host.Name)
			if err != nil {
				return err
			}
			if skip {
				return nil
			}
			importer := importers[host.Name]
			for _, target := range host.ListTarget() {
				switch importer.FileType(ctx, target.Target) {
				case imports.IsNotFound:
					logger.Info(fmt.Sprintf("%s is not found: skip", target.Target), zap.String("host", host.Name))
					continue
				case imports.IsFile:
					destination := filepath.Join(host.Name, target.Src)
					skip, err := checkImportDestination(repo, warnings, host.Name, destination)
					if err != nil {
						return err
					}
					if skip {
						continue
					}
					content, mode, err := importer.GetFileContent(ctx, target.Target)
					if err != nil {
						return err
					}
					if err := repo.CreateFile(destination, content, mode); err != nil {
						var symlink *localrepo.SymlinkInWritePath
						if errors.As(err, &symlink) {
							addImportSymlinkWarning(warnings, host.Name, symlink.Path)
							continue
						}
						return err
					}
				case imports.IsDirectory:
					files, err := importer.ListUntrackedFiles(ctx, target.Target)
					if err != nil {
						return err
					}
					files = importer.ExcludeSymlinkFiles(ctx, files)
					for _, file := range files {
						destination := filepath.Join(host.Name, target.Src, file)
						skip, err := checkImportDestination(repo, warnings, host.Name, destination)
						if err != nil {
							return err
						}
						if skip {
							continue
						}
						fileAbsPath := filepath.Join(target.Target, file)
						content, mode, err := importer.GetFileContent(ctx, fileAbsPath)
						if err != nil {
							return err
						}
						if err := repo.CreateFile(destination, content, mode); err != nil {
							var symlink *localrepo.SymlinkInWritePath
							if errors.As(err, &symlink) {
								addImportSymlinkWarning(warnings, host.Name, symlink.Path)
								continue
							}
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
	return &ImportResult{Result: Result{Hosts: hosts, Warnings: warnings.List()}}, err
}

func appendUnique(values []string, value string) []string {
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	return append(values, value)
}

func checkImportDestination(repo localrepo.LocalRepoIface, warnings *warningList, host, destination string) (bool, error) {
	err := repo.ValidateWritePath(destination)
	if err == nil {
		return false, nil
	}
	var symlink *localrepo.SymlinkInWritePath
	if !errors.As(err, &symlink) {
		return false, err
	}
	addImportSymlinkWarning(warnings, host, symlink.Path)
	return true, nil
}

func addImportSymlinkWarning(warnings *warningList, host, path string) {
	warnings.AddUnique(fmt.Sprintf("host %s: import skipped destination under symlink %q", host, path))
}
