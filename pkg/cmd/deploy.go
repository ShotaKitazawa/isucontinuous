package cmd

import (
	"context"
	"fmt"
	"time"

	"go.uber.org/zap"
	"k8s.io/utils/exec"

	"github.com/ShotaKitazawa/isucontinuous/pkg/config"
	"github.com/ShotaKitazawa/isucontinuous/pkg/hoststate"
	"github.com/ShotaKitazawa/isucontinuous/pkg/localrepo"
	"github.com/ShotaKitazawa/isucontinuous/pkg/shell"
	"github.com/ShotaKitazawa/isucontinuous/pkg/slack"
	"github.com/ShotaKitazawa/isucontinuous/pkg/template"
	"github.com/ShotaKitazawa/isucontinuous/pkg/usecases/deploy"
	"github.com/ShotaKitazawa/isucontinuous/pkg/usecases/lock"
)

type ConfigDeploy struct {
	ConfigCommon
	GitRevision string
	// Force takes over host locks held by someone else.
	Force bool
	// IgnoreDrift skips the check that files on hosts match the revision deployed last.
	IgnoreDrift bool
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
	return runDeploy(conf, ctx, logger, repo, newSlackClient, deploy.NewDeployers, lock.NewLockers)
}

func runDeploy(
	conf ConfigDeploy, ctx context.Context, logger *zap.Logger,
	repo localrepo.LocalRepoIface, newSlackClient func(*config.Config) slack.ClientIface,
	newDeployersFunc deploy.NewDeployersFunc, newLockersFunc lock.NewLockersFunc,
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
	// Get CommitHash & CommitMessage
	hash, msg, err := repo.GetHeadInfo(ctx)
	if err != nil {
		return nil, err
	}
	result := &DeployResult{Revision: conf.GitRevision, Hash: hash, Message: msg}
	result.Warnings = isucontinuous.Warnings
	owner, err := conf.owner()
	if err != nil {
		return result, err
	}
	// Set deployers
	deployers, err := newDeployersFunc(logger, template.New(hash), conf.LocalRepoPath, isucontinuous.Hosts)
	if err != nil {
		return result, err
	}
	lockers, err := newLockersFunc(logger, isucontinuous.Hosts)
	if err != nil {
		return result, err
	}
	// Lock all hosts or none
	want := hoststate.Lock{Owner: owner, Revision: hash, AcquiredAt: time.Now(), Operation: "deploy"}
	if _, err := acquireAll(ctx, logger, lockers, isucontinuous.Hosts, owner, want,
		hoststate.AcquireOptions{Force: conf.Force, Refresh: true}); err != nil {
		return result, err
	}
	if !conf.IgnoreDrift {
		if err := checkDrift(conf, ctx, logger, repo, isucontinuous.Hosts, deployers); err != nil {
			releaseAllOwn(ctx, logger, lockers, isucontinuous.Hosts, owner)
			return result, err
		}
	}
	// Deploy files to per host
	baseCtx := ctx
	hosts, err := perHostExec(logger, ctx, conf.reporter(), isucontinuous.Hosts, []task{{
		"Deploy",
		func(ctx context.Context, host config.Host) (err error) {
			// baseCtx, not ctx: ctx is cancelled when another host fails
			defer func() {
				if err != nil {
					releaseOwn(baseCtx, logger, lockers[host.Name], owner, host.Name)
				}
			}()
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
			return deployer.RecordRevision(ctx, hash)
		},
	}})
	result.Hosts = hosts
	return result, err
}

// checkDrift fails if a host has files that differ from the revision deployed last; hosts with no record are not checked.
func checkDrift(
	conf ConfigDeploy, ctx context.Context, logger *zap.Logger, repo localrepo.LocalRepoIface,
	hosts []config.Host, deployers map[string]*deploy.Deployer,
) error {
	diffs := make([]deploy.HostDiff, len(hosts))
	index := make(map[string]int, len(hosts))
	for idx, host := range hosts {
		index[host.Name] = idx
	}
	if _, err := perHostExec(logger, ctx, conf.reporter(), hosts, []task{{
		"Check drift",
		func(ctx context.Context, host config.Host) error {
			deployer := deployers[host.Name]
			last, found, err := deployer.LastRevision(ctx)
			if err != nil || !found {
				return err
			}
			d, err := deployer.Diff(ctx, host.Name, host.Deploy.Targets, last, repo, false)
			if err != nil {
				return err
			}
			diffs[index[host.Name]] = *d
			return nil
		},
	}}); err != nil {
		return err
	}
	var drifted []deploy.HostDiff
	for _, d := range diffs {
		if len(d.Changes) > 0 {
			drifted = append(drifted, d)
		}
	}
	if len(drifted) > 0 {
		return &deploy.DriftError{Diffs: drifted}
	}
	return nil
}
