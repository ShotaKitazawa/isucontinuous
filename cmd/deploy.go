package cmd

import (
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
				LogLevel:      logLevel,
				LogFilename:   logfile,
				LocalRepoPath: localRepo,
			},
			GitRevision: deployGitRevision,
			Force:       deployForce,
			SlackToken:  deploySlackToken,
		}
		return cmd.RunDeploy(conf)
	},
}

var (
	deployGitRevision string
	deployForce       bool
	deploySlackToken  string
)

func init() {
	rootCmd.AddCommand(deployCmd)
	deployCmd.PersistentFlags().StringVarP(&deployGitRevision, "revision", "b", "master",
		"branch-name, tag-name, or commit-hash of deployed from Git remote-repo")
	deployCmd.PersistentFlags().BoolVarP(&deployForce, "force", "f", false,
		"force deploy")
	deployCmd.PersistentFlags().StringVarP(&deploySlackToken, "slack-token", "t", "",
		"slack token of workspace where deployment notification will be sent")
	refStringEnvVarP(&deploySlackToken, "slack-token")
	_ = initCmd.MarkPersistentFlagRequired("slack-token")
}
