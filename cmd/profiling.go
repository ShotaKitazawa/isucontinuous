package cmd

import (
	"github.com/spf13/cobra"

	"github.com/ShotaKitazawa/isucontinuous/pkg/cmd"
)

// profilingCmd represents the profiling command
var profilingCmd = &cobra.Command{
	Use:   "profiling",
	Short: "Execute profiling command and wait to finish synchronously.",
	PreRunE: func(cmd *cobra.Command, args []string) error {
		return checkRequiredFlags(cmd.Flags())
	},
	RunE: func(c *cobra.Command, args []string) error {
		executed = true
		conf := cmd.ConfigProfiling{
			ConfigCommon: cmd.ConfigCommon{
				LogLevel:      logLevel,
				LogFilename:   logfile,
				LocalRepoPath: localRepo,
			},
		}
		return cmd.RunProfiling(conf)
	},
}

func init() {
	rootCmd.AddCommand(profilingCmd)
}
