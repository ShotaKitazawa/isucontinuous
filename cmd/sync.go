package cmd

import (
	"github.com/spf13/cobra"

	"github.com/ShotaKitazawa/isucontinuous/pkg/cmd"
)

// syncCmd represents the sync command
var syncCmd = &cobra.Command{
	Use:   "sync",
	Short: "synchronize local-repo with remote-repo",
	RunE: func(c *cobra.Command, args []string) error {
		executed = true
		conf := cmd.ConfigSync{
			ConfigCommon: cmd.ConfigCommon{
				LogLevel:      logLevel,
				LogFilename:   logfile,
				LocalRepoPath: localRepo,
			},
			GitBranch: syncGitBranch,
		}
		return cmd.RunSync(conf)
	},
}

var (
	syncGitBranch string
)

func init() {
	rootCmd.AddCommand(syncCmd)
	syncCmd.PersistentFlags().StringVarP(&syncGitBranch, "branch", "b", "master",
		"branch-name to push to Git remote-repo")
}
