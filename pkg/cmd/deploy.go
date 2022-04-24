package cmd

import (
	"context"
	"fmt"

	"go.uber.org/zap"
	"k8s.io/utils/exec"

	"github.com/ShotaKitazawa/isu-continuous/pkg/config"
	"github.com/ShotaKitazawa/isu-continuous/pkg/deploy"
	"github.com/ShotaKitazawa/isu-continuous/pkg/localrepo"
	"github.com/ShotaKitazawa/isu-continuous/pkg/shell"
	"github.com/ShotaKitazawa/isu-continuous/pkg/slack"
	"github.com/ShotaKitazawa/isu-continuous/pkg/template"
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
	// load isucontinuous.yaml
	isucontinuous, err := config.Load(conf.LocalRepoPath, isucontinuousFilename)
	if err != nil {
		return err
	}
	// set importers
	templator := template.New(conf.GitRevision)
	deployers := make(map[string]*deploy.Deployer)
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
		deployers[host.Host] = deploy.New(logger, s, templator)
	}
	slackClient := slack.NewClient(logger, conf.SlackToken, isucontinuous.Slack.DefaultChannel)
	return runDeploy(conf, ctx, logger, isucontinuous, deployers, slackClient)
}

func runDeploy(
	conf ConfigDeploy, ctx context.Context, logger *zap.Logger,
	isucontinuous *config.Config, deployers map[string]*deploy.Deployer,
	slackClient *slack.Client,
) error {
	// Attach local-repo
	repo, err := localrepo.AttachLocalRepo(logger, exec.New(), conf.LocalRepoPath)
	if err != nil {
		return err
	}
	// Fetch remote-repo & switch to gitRevision
	if err := repo.Fetch(); err != nil {
		return err
	}
	if err := repo.Switch(conf.GitRevision); err != nil {
		return err
	}
	// Deploy files to per host
	return perHostExec(logger, ctx, isucontinuous.Hosts, func(ctx context.Context, host config.Host) error {
		deployer := deployers[host.Host]
		var err error
		// Notify to Slack
		slackClient.SendText(ctx, host.Deploy.SlackChannel,
			fmt.Sprintf("**<%s> %s deploying...**", conf.GitRevision, host.Host))
		defer func() {
			if err != nil {
				slackClient.SendText(ctx, host.Deploy.SlackChannel,
					fmt.Sprintf("**<%s> %s deploy failed** :sob:", conf.GitRevision, host.Host))
			} else {
				slackClient.SendText(ctx, host.Deploy.SlackChannel,
					fmt.Sprintf("**<%s> %s deploy succeeded** :laughing:", conf.GitRevision, host.Host))
			}
		}()
		// Execute preCommand
		if err = deployer.RunCommand(host.Deploy.PreCommand); err != nil {
			return err
		}
		// Deploy
		if err = deployer.Deploy(host.Deploy.Targets); err != nil {
			return err
		}
		// Execute postCommand
		if err = deployer.RunCommand(host.Deploy.PostCommand); err != nil {
			return err
		}
		return nil
	})
}
