package cmd

import (
	"context"

	"go.uber.org/zap"
	"k8s.io/utils/exec"

	"github.com/ShotaKitazawa/isucontinuous/pkg/config"
	"github.com/ShotaKitazawa/isucontinuous/pkg/hoststate"
	"github.com/ShotaKitazawa/isucontinuous/pkg/localrepo"
	"github.com/ShotaKitazawa/isucontinuous/pkg/shell"
	"github.com/ShotaKitazawa/isucontinuous/pkg/usecases/lock"
)

type HostStatus struct {
	Host         string
	Disabled     bool            // not connected to
	Lock         *hoststate.Lock // nil if unlocked
	LastRevision string          // empty if none is recorded
}

type StatusResult struct {
	Result
	Statuses []HostStatus
}

type ConfigStatus struct {
	ConfigCommon
}

func RunStatus(conf ConfigStatus) (*StatusResult, error) {
	return RunStatusContext(context.Background(), conf)
}

func RunStatusContext(ctx context.Context, conf ConfigStatus) (*StatusResult, error) {
	logger, err := newLogger(conf.LogLevel, conf.LogFilename)
	if err != nil {
		return nil, err
	}
	shell.SetKnownHostsPath(conf.KnownHostsPath)
	repo, err := localrepo.AttachLocalRepo(logger, exec.New(), conf.LocalRepoPath)
	if err != nil {
		return nil, err
	}
	return runStatus(conf, ctx, logger, repo, lock.NewLockers)
}

func runStatus(
	conf ConfigStatus, ctx context.Context, logger *zap.Logger,
	repo localrepo.LocalRepoIface, newLockersFunc lock.NewLockersFunc,
) (*StatusResult, error) {
	logger.Info("start status")
	defer func() { logger.Info("finish status") }()
	isucontinuous, err := repo.LoadConf()
	if err != nil {
		return nil, err
	}
	lockers, err := newLockersFunc(logger, isucontinuous.Hosts)
	if err != nil {
		return nil, err
	}
	statuses := make([]HostStatus, len(isucontinuous.Hosts), len(isucontinuous.Hosts)+len(isucontinuous.Disabled))
	index := make(map[string]int, len(isucontinuous.Hosts))
	for idx, host := range isucontinuous.Hosts {
		index[host.Name] = idx
		statuses[idx] = HostStatus{Host: host.Name}
	}
	results, err := perHostExec(logger, ctx, conf.reporter(), isucontinuous.Hosts, []task{{
		"Status",
		func(ctx context.Context, host config.Host) error {
			holder, last, err := lockers[host.Name].Status(ctx)
			if err != nil {
				return err
			}
			statuses[index[host.Name]].Lock = holder
			statuses[index[host.Name]].LastRevision = last
			return nil
		},
	}})
	for _, d := range isucontinuous.Disabled {
		statuses = append(statuses, HostStatus{Host: d.Name, Disabled: true})
	}
	return &StatusResult{Result: Result{Hosts: results, Warnings: isucontinuous.Warnings}, Statuses: statuses}, err
}
