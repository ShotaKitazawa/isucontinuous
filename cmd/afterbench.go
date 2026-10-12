package cmd

import (
	"os"

	"github.com/spf13/cobra"

	"github.com/ShotaKitazawa/isucontinuous/pkg/cmd"
)

// afterbenchCmd represents the afterbench command
var afterbenchCmd = &cobra.Command{
	Use:   "afterbench",
	Short: "Collect and parse profile data & Send to Slack",
	PreRunE: func(cmd *cobra.Command, args []string) error {
		return checkRequiredFlags(cmd.Flags())
	},
	RunE: func(c *cobra.Command, args []string) error {
		executed = true
		conf := cmd.ConfigAfterBench{
			ConfigCommon: cmd.ConfigCommon{
				LogLevel:       logLevel,
				LogFilename:    logfile,
				LocalRepoPath:  localRepo,
				KnownHostsPath: knownHosts,
			},
			SlackToken: deploySlackToken,
		}
		r, err := cmd.RunAfterBench(conf)
		if r != nil {
			printWarnings(os.Stderr, r.Warnings)
		}
		return err
	},
}

var (
	afterbenchSlackToken string
)

func init() {
	rootCmd.AddCommand(afterbenchCmd)
	afterbenchCmd.PersistentFlags().StringVarP(&afterbenchSlackToken, "slack-token", "t", getenvDefault("SLACK_TOKEN", ""),
		"slack token of workspace where deployment notification will be sent")
	setRequired(afterbenchCmd, "slack-token")
}
