package cmd

import (
	"github.com/ShotaKitazawa/isucontinuous/pkg/cmd"
	"github.com/spf13/cobra"
)

// setupCmd represents the setup command
var setupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Install some softwares",
	RunE: func(c *cobra.Command, args []string) error {
		return wrapperFunc(cmd.RunSetup(
			cmd.ConfigSetup{
				ConfigCommon: cmd.ConfigCommon{
					LogFilename:   logfile,
					LocalRepoPath: localRepo,
				},
			},
		))
	},
}

func init() {
	rootCmd.AddCommand(setupCmd)
}
