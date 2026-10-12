package cmd

import (
	"context"
	"fmt"

	"go.uber.org/zap"
	"k8s.io/utils/exec"

	"github.com/ShotaKitazawa/isucontinuous/pkg/config"
	"github.com/ShotaKitazawa/isucontinuous/pkg/hoststate"
	"github.com/ShotaKitazawa/isucontinuous/pkg/localrepo"
	"github.com/ShotaKitazawa/isucontinuous/pkg/shell"
	"github.com/ShotaKitazawa/isucontinuous/pkg/usecases/lock"
)

type ReleaseState string

const (
	ReleaseReleased    ReleaseState = "released"
	ReleaseNotLocked   ReleaseState = "not locked"
	ReleaseHeldByOther ReleaseState = "held by another owner"
)

type HostRelease struct {
	Host   string
	State  ReleaseState
	Holder *hoststate.Lock // the lock that was found, if any
}

type ReleaseResult struct {
	Result
	Releases []HostRelease
}

type ConfigRelease struct {
	ConfigCommon
	// Force releases locks held by someone else.
	Force bool
	// IncludeDisabled also releases the locks of disabled hosts.
	IncludeDisabled bool
}

func RunRelease(conf ConfigRelease) (*ReleaseResult, error) {
	ctx := context.Background()
	logger, err := newLogger(conf.LogLevel, conf.LogFilename)
	if err != nil {
		return nil, err
	}
	shell.SetKnownHostsPath(conf.KnownHostsPath)
	repo, err := localrepo.AttachLocalRepo(logger, exec.New(), conf.LocalRepoPath)
	if err != nil {
		return nil, err
	}
	return runRelease(conf, ctx, logger, repo, lock.NewLockers)
}

func runRelease(
	conf ConfigRelease, ctx context.Context, logger *zap.Logger,
	repo localrepo.LocalRepoIface, newLockersFunc lock.NewLockersFunc,
) (*ReleaseResult, error) {
	logger.Info("start release")
	defer func() { logger.Info("finish release") }()
	isucontinuous, err := repo.LoadConf()
	if err != nil {
		return nil, err
	}
	owner, err := conf.owner()
	if err != nil {
		return nil, err
	}
	hosts := append([]config.Host(nil), isucontinuous.Hosts...)
	if conf.IncludeDisabled {
		for _, d := range isucontinuous.Disabled {
			if d.Err != nil {
				return nil, fmt.Errorf("disabled host %s cannot be reached: %w", d.Name, d.Err)
			}
			hosts = append(hosts, d.Host)
		}
	}
	lockers, err := newLockersFunc(logger, hosts)
	if err != nil {
		return nil, err
	}
	releases := make([]HostRelease, len(hosts))
	index := make(map[string]int, len(hosts))
	for idx, host := range hosts {
		index[host.Name] = idx
		releases[idx] = HostRelease{Host: host.Name}
	}
	results, err := perHostExec(logger, ctx, conf.reporter(), hosts, []task{{
		"Release",
		func(ctx context.Context, host config.Host) error {
			outcome, holder, err := lockers[host.Name].Release(ctx, owner, conf.Force)
			if err != nil {
				return err
			}
			r := &releases[index[host.Name]]
			r.Holder = holder
			switch outcome {
			case hoststate.Released:
				r.State = ReleaseReleased
			case hoststate.HeldByOther:
				r.State = ReleaseHeldByOther
			default:
				r.State = ReleaseNotLocked
			}
			return nil
		},
	}})
	return &ReleaseResult{Result: Result{Hosts: results, Warnings: isucontinuous.Warnings}, Releases: releases}, err
}
