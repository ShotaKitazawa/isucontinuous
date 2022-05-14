package cmd

import (
	"context"
	"fmt"
	"time"

	"go.uber.org/zap"
	"k8s.io/utils/exec"

	"github.com/ShotaKitazawa/isucontinuous/pkg/config"
	"github.com/ShotaKitazawa/isucontinuous/pkg/localrepo"
	"github.com/ShotaKitazawa/isucontinuous/pkg/slack"
	"github.com/ShotaKitazawa/isucontinuous/pkg/template"
	"github.com/ShotaKitazawa/isucontinuous/pkg/usecases/afterbench"
)

type ConfigAfterBench struct {
	ConfigCommon
	SlackToken string
}

func RunAfterBench(conf ConfigAfterBench) error {
	ctx := context.Background()
	logger, err := newLogger(conf.LogLevel, conf.LogFilename)
	if err != nil {
		return err
	}
	// Attach local-repo
	repo, err := localrepo.AttachLocalRepo(logger, exec.New(), conf.LocalRepoPath)
	if err != nil {
		return err
	}
	// load isucontinuous.yaml
	isucontinuous, err := repo.LoadConf()
	if err != nil {
		return err
	}
	slackClient := slack.NewClient(logger, conf.SlackToken, isucontinuous.Slack.DefaultChannelId)
	return runAfterBench(conf, ctx, logger, repo, slackClient, afterbench.New)
}

func runAfterBench(
	conf ConfigAfterBench, ctx context.Context, logger *zap.Logger,
	repo localrepo.LocalRepoIface, slackClient slack.ClientIface,
	newAfterBenchersFunc afterbench.NewFunc,
) error {
	logger.Info("start afterbench")
	defer func() { logger.Info("finish afterbench") }()
	// Load isucontinus.yaml
	isucontinuous, err := repo.LoadConf()
	if err != nil {
		return err
	}
	// Get revision
	gitRevision, err := repo.GetRevision(ctx)
	if err != nil {
		return fmt.Errorf("%s/.revision is not found. exec `deploy` command first", conf.LocalRepoPath)
	}
	// AfterBench files to per host
	if err := perHostExec(logger, ctx, isucontinuous.Hosts, []task{{
		"AfterBench",
		func(ctx context.Context, host config.Host) error {
			if host.AfterBench.Target == "" {
				logger.Debug("skip bacause target is not specified", zap.String("host", host.Host))
				return nil
			}
			afterbencher, err := newAfterBenchersFunc(logger, template.New(gitRevision), slackClient, host)
			if err != nil {
				return err
			}
			// execute to collect & parse profile data
			if err := afterbencher.RunCommand(ctx, host.AfterBench.Command); err != nil {
				return err
			}
			// cleanup some profile data
			defer func() {
				_ = afterbencher.CleanUp(ctx, host.AfterBench.Target, fmt.Sprintf("%d", time.Now().Unix()))
			}()
			// post profile data to Slack
			if err := afterbencher.PostToSlack(ctx, host.AfterBench.Target, host.AfterBench.SlackChannelId); err != nil {
				return err
			}
			return nil
		}}}); err != nil {
		return err
	}
	// Clear revision-file
	return repo.ClearRevision(ctx)
}
