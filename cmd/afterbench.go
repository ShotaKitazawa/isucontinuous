package cmd

import (
	"github.com/spf13/cobra"

	"github.com/ShotaKitazawa/isucontinuous/pkg/cmd"
)

// afterbenchCmd represents the afterbench command
var afterbenchCmd = &cobra.Command{
	Use:   "afterbench",
	Short: "Collect and parse profile data & Send to Slack",
	RunE: func(c *cobra.Command, args []string) error {
		executed = true
		conf := cmd.ConfigAfterBench{
			ConfigCommon: cmd.ConfigCommon{
				LogLevel:      logLevel,
				LogFilename:   logfile,
				LocalRepoPath: localRepo,
			},
			SlackToken: deploySlackToken,
		}
		return cmd.RunAfterBench(conf)
	},
}

func init() {
	rootCmd.AddCommand(afterbenchCmd)
	afterbenchCmd.PersistentFlags().StringVarP(&deploySlackToken, "slack-token", "t", "",
		"slack token of workspace where deployment notification will be sent")
}
