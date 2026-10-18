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
	RunE: func(c *cobra.Command, args []string) error {
		executed = true
		conf := cmd.ConfigAfterBench{
			ConfigCommon: cmd.ConfigCommon{
				LogLevel:       logLevel,
				LogFilename:    logfile,
				LocalRepoPath:  localRepo,
				KnownHostsPath: knownHosts,
			},
			SlackToken: afterbenchSlackToken,
		}
		r, err := cmd.RunAfterBenchContext(c.Context(), conf)
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
		"slack token of workspace where profile data will be sent (used only when slack.default_channel_id is set)")
}
