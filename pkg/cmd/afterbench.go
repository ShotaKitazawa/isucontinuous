package cmd

import (
	"context"
	"fmt"
	"time"

	"go.uber.org/zap"
	"k8s.io/utils/exec"

	"github.com/ShotaKitazawa/isucontinuous/pkg/config"
	"github.com/ShotaKitazawa/isucontinuous/pkg/localrepo"
	"github.com/ShotaKitazawa/isucontinuous/pkg/shell"
	"github.com/ShotaKitazawa/isucontinuous/pkg/template"
	"github.com/ShotaKitazawa/isucontinuous/pkg/usecases/afterbench"
	"github.com/ShotaKitazawa/isucontinuous/pkg/usecases/lock"
)

type ConfigAfterBench struct {
	ConfigCommon
	SlackToken string
}

func RunAfterBench(conf ConfigAfterBench) (*AfterBenchResult, error) {
	return RunAfterBenchContext(context.Background(), conf)
}

func RunAfterBenchContext(ctx context.Context, conf ConfigAfterBench) (*AfterBenchResult, error) {
	logger, err := newLogger(conf.LogLevel, conf.LogFilename)
	if err != nil {
		return nil, err
	}
	shell.SetKnownHostsPath(conf.KnownHostsPath)
	// Attach local-repo
	repo, err := localrepo.AttachLocalRepo(logger, exec.New(), conf.LocalRepoPath)
	if err != nil {
		return nil, err
	}
	unlock, err := repo.Lock(ctx)
	if err != nil {
		return nil, err
	}
	defer unlock()
	return runAfterBench(conf, ctx, logger, repo, realSlackClient(logger, conf.SlackToken), afterbench.New, lock.NewLockers)
}

func runAfterBench(
	conf ConfigAfterBench, ctx context.Context, logger *zap.Logger,
	repo localrepo.LocalRepoIface, newSlackClient newSlackClientFunc,
	newAfterBenchersFunc afterbench.NewFunc, newLockersFunc lock.NewLockersFunc,
) (*AfterBenchResult, error) {
	logger.Info("start afterbench")
	defer func() { logger.Info("finish afterbench") }()
	// Load isucontinus.yaml
	isucontinuous, err := repo.LoadConf()
	if err != nil {
		return nil, err
	}
	owner, err := conf.owner()
	if err != nil {
		return nil, err
	}
	lockers, err := newLockersFunc(logger, isucontinuous.Hosts)
	if err != nil {
		return nil, err
	}
	locks, err := requireAll(ctx, logger, lockers, isucontinuous.Hosts, owner)
	if err != nil {
		return nil, err
	}
	if err := checkHeadMatchesLocks(ctx, repo, isucontinuous.Hosts, locks); err != nil {
		return nil, err
	}
	// AfterBench files to per host
	warnings := &warningList{}
	for _, w := range isucontinuous.Warnings {
		warnings.Add(w)
	}
	slackClient := slackNotifier(logger, isucontinuous, newSlackClient, warnings)
	baseCtx := ctx
	perHost := make([][]Artifact, len(isucontinuous.Hosts))
	hostIndex := make(map[string]int, len(isucontinuous.Hosts))
	for idx, host := range isucontinuous.Hosts {
		hostIndex[host.Name] = idx
	}
	hosts, err := perHostExec(logger, ctx, conf.reporter(), isucontinuous.Hosts, []task{{
		"AfterBench",
		func(ctx context.Context, host config.Host) (err error) {
			// a failed host keeps its lock so that afterbench can run again: the logs are cleared by the next deploy
			defer func() {
				if err == nil {
					cleanupCtx, cancel := lockCleanupContext(baseCtx)
					defer cancel()
					releaseOwn(cleanupCtx, logger, lockers[host.Name], owner, host.Name)
				}
			}()
			if host.AfterBench.Target == "" {
				logger.Debug("skip bacause target is not specified", zap.String("host", host.Name))
				return nil
			}
			afterbencher, err := newAfterBenchersFunc(logger, template.New(locks[host.Name].Revision), slackClient, host)
			if err != nil {
				return err
			}
			// mkdir location to deploy profile data
			if err := afterbencher.Prepare(ctx, host.AfterBench.Target); err != nil {
				return err
			}
			// cleanup location of profile data
			defer func() {
				suffix := fmt.Sprintf("%d", time.Now().Unix())
				moved, err := afterbencher.CleanUp(ctx, host.AfterBench.Target, suffix)
				if err != nil {
					msg := fmt.Sprintf("[%s] failed to cleanup: %v", host.Name, err)
					logger.Error(msg, zap.String("host", host.Name))
					warnings.Add(msg)
				}
				// the paths reported are those on the server after the move; without it they stay where they were written
				for idx := range perHost[hostIndex[host.Name]] {
					a := &perHost[hostIndex[host.Name]][idx]
					a.Path = moved.Path(a.Path)
				}
			}()

			// execute to collect & parse profile data
			if err := afterbencher.RunCommand(ctx, host.AfterBench.Command); err != nil {
				return err
			}
			// post profile data to Slack
			artifacts, err := afterbencher.PostToSlack(ctx, host.AfterBench.Target, host.AfterBench.SlackChannelId)
			for _, a := range artifacts {
				perHost[hostIndex[host.Name]] = append(perHost[hostIndex[host.Name]],
					Artifact{Host: host.Name, Path: a.Path, Size: len(a.Content), Content: a.Content})
			}
			return err
		}}})
	result := &AfterBenchResult{Result: Result{Hosts: hosts, Warnings: warnings.List()}}
	for _, as := range perHost {
		result.Artifacts = append(result.Artifacts, as...)
	}
	return result, err
}
