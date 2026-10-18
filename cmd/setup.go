package cmd

import (
	"os"

	"github.com/spf13/cobra"

	"github.com/ShotaKitazawa/isucontinuous/pkg/cmd"
)

// setupCmd represents the setup command
var setupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Install some softwares",
	PreRunE: func(cmd *cobra.Command, args []string) error {
		return checkRequiredFlags(cmd.Flags())
	},
	RunE: func(c *cobra.Command, args []string) error {
		executed = true
		conf := cmd.ConfigSetup{
			ConfigCommon: cmd.ConfigCommon{
				LogLevel:       logLevel,
				LogFilename:    logfile,
				LocalRepoPath:  localRepo,
				KnownHostsPath: knownHosts,
			},
		}
		r, err := cmd.RunSetupContext(c.Context(), conf)
		if r != nil {
			printWarnings(os.Stderr, r.Warnings)
		}
		return err
	},
}

func init() {
	rootCmd.AddCommand(setupCmd)
}
