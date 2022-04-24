package cmd

import (
	"github.com/spf13/cobra"

	"github.com/ShotaKitazawa/isu-continuous/pkg/cmd"
)

// setupCmd represents the setup command
var setupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Install some softwares",
	RunE: func(c *cobra.Command, args []string) error {
		executed = true
		conf := cmd.ConfigSetup{
			ConfigCommon: cmd.ConfigCommon{
				LogLevel:      logLevel,
				LogFilename:   logfile,
				LocalRepoPath: localRepo,
			},
		}
		return cmd.RunSetup(conf)
	},
}

func init() {
	rootCmd.AddCommand(setupCmd)
}
