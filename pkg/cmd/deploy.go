package cmd

import (
	"context"
	"fmt"

	"go.uber.org/zap"
	"k8s.io/utils/exec"

	"github.com/ShotaKitazawa/isucontinuous/pkg/config"
	"github.com/ShotaKitazawa/isucontinuous/pkg/localrepo"
	"github.com/ShotaKitazawa/isucontinuous/pkg/shell"
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

func RunDeploy(conf ConfigDeploy) (*DeployResult, error) {
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
	newSlackClient := func(c *config.Config) slack.ClientIface {
		return slack.NewClient(logger, conf.SlackToken, c.Slack.DefaultChannelId)
	}
	return runDeploy(conf, ctx, logger, repo, newSlackClient, deploy.NewDeployers)
}

func runDeploy(
	conf ConfigDeploy, ctx context.Context, logger *zap.Logger,
	repo localrepo.LocalRepoIface, newSlackClient func(*config.Config) slack.ClientIface,
	newDeployersFunc deploy.NewDeployersFunc,
) (*DeployResult, error) {
	logger.Info("start deploy")
	defer func() { logger.Info("finish deploy") }()
	// Fetch remote-repo & switch to gitRevision
	if err := repo.Fetch(ctx); err != nil {
		return nil, err
	}
	if err := repo.SwitchDetachedBranch(ctx, conf.GitRevision); err != nil {
		return nil, err
	}
	// Load isucontinus.yaml
	isucontinuous, err := repo.LoadConf()
	if err != nil {
		return nil, err
	}
	slackClient := newSlackClient(isucontinuous)
	// Check to have already deployed
	if !conf.Force {
		if r, err := repo.GetRevision(ctx); err == nil {
			if r != conf.GitRevision {
				return nil, fmt.Errorf(
					`"deploy" command has already been executed. Please execute "afterbench" or "deploy --force".`)
			}
		}
	}
	// Get CommitHash & CommitMessage
	hash, msg, err := repo.GetHeadInfo(ctx)
	if err != nil {
		return nil, err
	}
	result := &DeployResult{Revision: conf.GitRevision, Hash: hash, Message: msg}
	result.Warnings = isucontinuous.Warnings
	// Set deployers
	deployers, err := newDeployersFunc(logger, template.New(conf.GitRevision), conf.LocalRepoPath, isucontinuous.Hosts)
	if err != nil {
		return result, err
	}
	// Deploy files to per host
	hosts, err := perHostExec(logger, ctx, conf.reporter(), isucontinuous.Hosts, []task{{
		"Deploy",
		func(ctx context.Context, host config.Host) error {
			// Notify to Slack
			if err := slackClient.SendText(ctx, host.Deploy.SlackChannelId,
				fmt.Sprintf("*<%s> deploying to %s...*", conf.GitRevision, host.Name)); err != nil {
				return err
			}
			defer func() {
				if err != nil {
					_ = slackClient.SendText(ctx, host.Deploy.SlackChannelId,
						fmt.Sprintf("*<%s> deploy was failed to %s* :sob:", conf.GitRevision, host.Name))
				} else {
					_ = slackClient.SendText(ctx, host.Deploy.SlackChannelId,
						fmt.Sprintf("*<%s> deploy was succeeded to %s* :laughing:", conf.GitRevision, host.Name))
				}
			}()
			deployer := deployers[host.Name]
			// Execute preCommand
			if err = deployer.RunCommand(ctx, host.Deploy.PreCommand); err != nil {
				return err
			}
			// Deploy
			if err = deployer.Deploy(ctx, host.Name, host.Deploy.Targets); err != nil {
				return err
			}
			// Execute postCommand
			if err = deployer.RunCommand(ctx, host.Deploy.PostCommand); err != nil {
				return err
			}
			// Record the revision that this host was deployed with
			if err = deployer.RecordRevision(ctx, hash); err != nil {
				return err
			}
			return nil
		},
	}})
	result.Hosts = hosts
	if err != nil {
		return result, err
	}
	// Store revision to local-repo
	return result, repo.SetRevision(ctx, conf.GitRevision)
}
