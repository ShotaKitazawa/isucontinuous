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
	"github.com/ShotaKitazawa/isucontinuous/pkg/slack"
	"github.com/ShotaKitazawa/isucontinuous/pkg/template"
	"github.com/ShotaKitazawa/isucontinuous/pkg/usecases/afterbench"
	"github.com/ShotaKitazawa/isucontinuous/pkg/usecases/lock"
)

type ConfigAfterBench struct {
	ConfigCommon
	SlackToken string
}

func RunAfterBench(conf ConfigAfterBench) (*AfterBenchResult, error) {
	ctx := context.Background()
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
	// load isucontinuous.yaml
	isucontinuous, err := repo.LoadConf()
	if err != nil {
		return nil, err
	}
	slackClient := slack.NewClient(logger, conf.SlackToken, isucontinuous.Slack.DefaultChannelId)
	return runAfterBench(conf, ctx, logger, repo, slackClient, afterbench.New, lock.NewLockers)
}

func runAfterBench(
	conf ConfigAfterBench, ctx context.Context, logger *zap.Logger,
	repo localrepo.LocalRepoIface, slackClient slack.ClientIface,
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
	baseCtx := ctx
	hosts, err := perHostExec(logger, ctx, conf.reporter(), isucontinuous.Hosts, []task{{
		"AfterBench",
		func(ctx context.Context, host config.Host) (err error) {
			// a failed host keeps its lock so that afterbench can run again: the logs are cleared by the next deploy
			defer func() {
				if err == nil {
					releaseOwn(baseCtx, logger, lockers[host.Name], owner, host.Name)
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
				if err := afterbencher.CleanUp(ctx, host.AfterBench.Target, suffix); err != nil {
					msg := fmt.Sprintf("[%s] failed to cleanup: %v", host.Name, err)
					logger.Error(msg, zap.String("host", host.Name))
					warnings.Add(msg)
				}
			}()

			// execute to collect & parse profile data
			if err := afterbencher.RunCommand(ctx, host.AfterBench.Command); err != nil {
				return err
			}
			// post profile data to Slack
			if err := afterbencher.PostToSlack(ctx, host.AfterBench.Target, host.AfterBench.SlackChannelId); err != nil {
				return err
			}
			return nil
		}}})
	return &AfterBenchResult{Result{Hosts: hosts, Warnings: warnings.List()}}, err
}
