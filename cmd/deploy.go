package cmd

import (
	"os"

	"github.com/ShotaKitazawa/isucontinuous/pkg/cmd"
	"github.com/spf13/cobra"
)

// deployCmd represents the deploy command
var deployCmd = &cobra.Command{
	Use:   "deploy",
	Short: "Deploy files from specified revision",
	RunE: func(c *cobra.Command, args []string) error {
		executed = true
		conf := cmd.ConfigDeploy{
			ConfigCommon: cmd.ConfigCommon{
				LogLevel:       logLevel,
				LogFilename:    logfile,
				LocalRepoPath:  localRepo,
				KnownHostsPath: knownHosts,
			},
			GitRevision: deployGitRevision,
			Force:       deployForce,
			IgnoreDrift: deployIgnoreDrift,
			SlackToken:  deploySlackToken,
		}
		r, err := cmd.RunDeploy(conf)
		printDeployResult(os.Stdout, r)
		if r != nil {
			printWarnings(os.Stderr, r.Warnings)
		}
		return err
	},
}

var (
	deployGitRevision string
	deployForce       bool
	deployIgnoreDrift bool
	deploySlackToken  string
)

func init() {
	rootCmd.AddCommand(deployCmd)
	deployCmd.PersistentFlags().StringVarP(&deployGitRevision, "revision", "b", "master",
		"branch-name, tag-name, or commit-hash of deployed from Git remote-repo")
	deployCmd.PersistentFlags().BoolVarP(&deployForce, "force", "f", false,
		"take over host locks held by someone else (check with the holder first)")
	deployCmd.PersistentFlags().BoolVar(&deployIgnoreDrift, "ignore-drift", false,
		"deploy even if files on hosts differ from the revision deployed last")
	deployCmd.PersistentFlags().StringVarP(&deploySlackToken, "slack-token", "t", getenvDefault("slack-token", ""),
		"slack token of workspace where deployment notification will be sent (used only when slack.default_channel_id is set)")
}
