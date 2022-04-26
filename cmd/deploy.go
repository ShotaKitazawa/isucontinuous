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
		}
		return cmd.RunDeploy(conf)

	},
}

var (
	gitRevision string
)

func init() {
	rootCmd.AddCommand(deployCmd)
	initCmd.PersistentFlags().StringVarP(&gitRevision, "revision", "s", "main",
		"branch-name, tag-name, or commit-hash of deployed from Git remote-repo")
}