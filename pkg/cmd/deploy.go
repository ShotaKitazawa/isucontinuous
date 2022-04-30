package cmd

import (
	"context"
	"fmt"

	"go.uber.org/zap"
	"k8s.io/utils/exec"

	"github.com/ShotaKitazawa/isu-continuous/pkg/config"
	"github.com/ShotaKitazawa/isu-continuous/pkg/localrepo"
	"github.com/ShotaKitazawa/isu-continuous/pkg/slack"
	"github.com/ShotaKitazawa/isu-continuous/pkg/template"
	"github.com/ShotaKitazawa/isu-continuous/pkg/usecases/deploy"
)

type ConfigDeploy struct {
	ConfigCommon
	GitRevision string
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
	isucontinuous, err := repo.LoadConf(isucontinuousFilename)
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
	// Fetch remote-repo & switch to gitRevision
	if err := repo.Fetch(ctx); err != nil {
		return err
	}
	if err := repo.SwitchDetachedBranch(ctx, conf.GitRevision); err != nil {
		return err
	}
	// Load isucontinus.yaml
	isucontinuous, err := repo.LoadConf(isucontinuousFilename)
	if err != nil {
		return err
	}
	// Set deployers
	deployers, err := newDeployersFunc(logger, template.New(conf.GitRevision), conf.LocalRepoPath, isucontinuous.Hosts)
	if err != nil {
		return err
	}
	// Deploy files to per host
	return perHostExec(logger, ctx, isucontinuous.Hosts, func(ctx context.Context, host config.Host) error {
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
	})
}
