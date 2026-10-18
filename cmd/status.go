package cmd

import (
	"os"

	"github.com/spf13/cobra"

	"github.com/ShotaKitazawa/isucontinuous/pkg/cmd"
)

// statusCmd represents the status command
var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show the lock holder and the revision deployed last of each host",
	PreRunE: func(cmd *cobra.Command, args []string) error {
		return checkRequiredFlags(cmd.Flags())
	},
	RunE: func(c *cobra.Command, args []string) error {
		executed = true
		conf := cmd.ConfigStatus{
			ConfigCommon: cmd.ConfigCommon{
				LogLevel:       logLevel,
				LogFilename:    logfile,
				LocalRepoPath:  localRepo,
				KnownHostsPath: knownHosts,
			},
		}
		r, err := cmd.RunStatusContext(c.Context(), conf)
		printStatusResult(os.Stdout, r)
		if r != nil {
			printWarnings(os.Stderr, r.Warnings)
		}
		return err
	},
}

func init() {
	rootCmd.AddCommand(statusCmd)
}
