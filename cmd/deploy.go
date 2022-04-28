/*
Copyright © 2022 NAME HERE <EMAIL ADDRESS>

*/
package cmd

import (
	"github.com/ShotaKitazawa/isu-continuous/pkg/cmd"
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
			GitRevision: gitRevision,
			SlackToken:  slackToken,
		}
		return cmd.RunDeploy(conf)
	},
}

var (
	gitRevision string
	slackToken  string
)

func init() {
	rootCmd.AddCommand(deployCmd)
	deployCmd.PersistentFlags().StringVarP(&gitRevision, "revision", "b", "master",
		"branch-name, tag-name, or commit-hash of deployed from Git remote-repo")
	deployCmd.PersistentFlags().StringVarP(&slackToken, "slack-token", "t", "",
		"slack token of workspace where deployment notification will be sent")
	_ = initCmd.MarkPersistentFlagRequired("slack-token")
}
