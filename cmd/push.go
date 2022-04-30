package cmd

import (
	"github.com/ShotaKitazawa/isu-continuous/pkg/cmd"
	"github.com/spf13/cobra"
)

// pushCmd represents the push command
var pushCmd = &cobra.Command{
	Use:   "push",
	Short: "Push local-repo to origin/${MAIN_BRANCH}",
	RunE: func(c *cobra.Command, args []string) error {
		executed = true
		conf := cmd.ConfigPush{
			ConfigCommon: cmd.ConfigCommon{
				LogLevel:      logLevel,
				LogFilename:   logfile,
				LocalRepoPath: localRepo,
			},
			GitBranch: gitBranch,
		}
		return cmd.RunPush(conf)
	},
}

var (
	gitBranch string
)

func init() {
	rootCmd.AddCommand(pushCmd)
	pushCmd.PersistentFlags().StringVarP(&gitBranch, "branch", "b", "master",
		"branch-name to push to Git remote-repo")

}