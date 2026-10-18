package cmd

import (
	"os"

	"github.com/spf13/cobra"

	"github.com/ShotaKitazawa/isucontinuous/pkg/cmd"
)

// releaseCmd represents the release command
var releaseCmd = &cobra.Command{
	Use:   "release",
	Short: "Release the host locks you hold",
	PreRunE: func(cmd *cobra.Command, args []string) error {
		return checkRequiredFlags(cmd.Flags())
	},
	RunE: func(c *cobra.Command, args []string) error {
		executed = true
		conf := cmd.ConfigRelease{
			ConfigCommon: cmd.ConfigCommon{
				LogLevel:       logLevel,
				LogFilename:    logfile,
				LocalRepoPath:  localRepo,
				KnownHostsPath: knownHosts,
			},
			Force:           releaseForce,
			IncludeDisabled: releaseIncludeDisabled,
		}
		r, err := cmd.RunReleaseContext(c.Context(), conf)
		printReleaseResult(os.Stdout, r)
		if r != nil {
			printWarnings(os.Stderr, r.Warnings)
		}
		return err
	},
}

var (
	releaseForce           bool
	releaseIncludeDisabled bool
)

func init() {
	rootCmd.AddCommand(releaseCmd)
	releaseCmd.PersistentFlags().BoolVarP(&releaseForce, "force", "f", false,
		"also release locks held by someone else (check with the holder first)")
	releaseCmd.PersistentFlags().BoolVar(&releaseIncludeDisabled, "include-disabled", false,
		"also release the locks of disabled hosts")
}
