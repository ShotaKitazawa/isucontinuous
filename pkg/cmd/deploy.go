package cmd

import (
	"context"
	"fmt"

	"go.uber.org/zap"
	"k8s.io/utils/exec"

	"github.com/ShotaKitazawa/isucontinuous/pkg/config"
	"github.com/ShotaKitazawa/isucontinuous/pkg/localrepo"
	"github.com/ShotaKitazawa/isucontinuous/pkg/slack"
	"github.com/ShotaKitazawa/isucontinuous/pkg/template"
	"github.com/ShotaKitazawa/isucontinuous/pkg/usecases/deploy"
)

type ConfigDeploy struct {
	ConfigCommon
	GitRevision string
	Force       bool
	SlackToken  string
}

func RunDeploy(conf ConfigDeploy) error {
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
	return runDeploy(conf, ctx, logger, repo, slackClient, deploy.NewDeployers)
}

func runDeploy(
	conf ConfigDeploy, ctx context.Context, logger *zap.Logger,
	repo localrepo.LocalRepoIface, slackClient slack.ClientIface,
	newDeployersFunc deploy.NewDeployersFunc,
) error {
	logger.Info("start deploy")
	defer func() { logger.Info("finish deploy") }()
	// Fetch remote-repo & switch to gitRevision
	if err := repo.Fetch(ctx); err != nil {
		return err
	}
	if err := repo.SwitchDetachedBranch(ctx, conf.GitRevision); err != nil {
		return err
	}
	// Load isucontinus.yaml
	isucontinuous, err := repo.LoadConf()
	if err != nil {
		return err
	}
	// Check to have already deployed
	if !conf.Force {
		if r, err := repo.GetRevision(ctx); err == nil {
			if r != conf.GitRevision {
				return fmt.Errorf(
					`"deploy" command has already been executed. Please execute "afterbench" or "deploy --force".`)
			}
		}
	}
	// Set deployers
	deployers, err := newDeployersFunc(logger, template.New(conf.GitRevision), conf.LocalRepoPath, isucontinuous.Hosts)
	if err != nil {
		return err
	}
	// Deploy files to per host
	if err := perHostExec(logger, ctx, isucontinuous.Hosts, func(ctx context.Context, host config.Host) error {
		deployer := deployers[host.Host]
		var err error
		// Notify to Slack
		if err := slackClient.SendText(ctx, host.Deploy.SlackChannel,
			fmt.Sprintf("*<%s> %s deploying...*", conf.GitRevision, host.Host)); err != nil {
			return err
		}
		defer func() {
			if err != nil {
				_ = slackClient.SendText(ctx, host.Deploy.SlackChannel,
					fmt.Sprintf("*<%s> %s deploy failed* :sob:", conf.GitRevision, host.Host))
			} else {
				_ = slackClient.SendText(ctx, host.Deploy.SlackChannel,
					fmt.Sprintf("*<%s> %s deploy succeeded* :laughing:", conf.GitRevision, host.Host))
			}
		}()
		// Execute preCommand
		if err = deployer.RunCommand(ctx, host.Deploy.PreCommand); err != nil {
			return err
		}
		// Deploy
		if err = deployer.Deploy(ctx, host.Deploy.Targets); err != nil {
			return err
		}
		// Execute postCommand
		if err = deployer.RunCommand(ctx, host.Deploy.PostCommand); err != nil {
			return err
		}
		return nil
	}); err != nil {
		return err
	}
	// Store revision to local-repo
	return repo.SetRevision(ctx, conf.GitRevision)
}
