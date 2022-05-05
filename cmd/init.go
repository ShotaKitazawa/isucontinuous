package cmd

import (
	"github.com/spf13/cobra"

	"github.com/ShotaKitazawa/isucontinuous/pkg/cmd"
)

// initCmd represents the init command
var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize local repository",
	RunE: func(c *cobra.Command, args []string) error {
		executed = true
		conf := cmd.ConfigInit{
			ConfigCommon: cmd.ConfigCommon{
				LogLevel:      logLevel,
				LogFilename:   logfile,
				LocalRepoPath: localRepo,
			},
			GitUsername:  gitUsername,
			GitEmail:     gitEmail,
			GitRemoteUrl: gitRemoteUrl,
		}
		return cmd.RunInit(conf)
	},
}

var (
	gitUsername  string
	gitEmail     string
	gitRemoteUrl string
)

func init() {
	rootCmd.AddCommand(initCmd)

	initCmd.PersistentFlags().StringVarP(&gitUsername, "username", "u", "isucontinuous",
		"username of GitHub Account")
	initCmd.PersistentFlags().StringVarP(&gitEmail, "email", "e", "isucontinuous@users.noreply.github.com",
		"email of GitHub Account")
	initCmd.PersistentFlags().StringVarP(&gitRemoteUrl, "remote-url", "r", getenvDefault("REMOTE_URL", ""),
		"URL of remote repository (requirement)")
	_ = initCmd.MarkPersistentFlagRequired("remote-url")
}
