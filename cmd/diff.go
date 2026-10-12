package cmd

import (
	"os"

	"github.com/spf13/cobra"

	"github.com/ShotaKitazawa/isucontinuous/pkg/cmd"
)

// diffCmd represents the diff command
var diffCmd = &cobra.Command{
	Use:   "diff",
	Short: "Show files on hosts whose content differs from a revision (default: the revision deployed last)",
	PreRunE: func(cmd *cobra.Command, args []string) error {
		return checkRequiredFlags(cmd.Flags())
	},
	RunE: func(c *cobra.Command, args []string) error {
		executed = true
		conf := cmd.ConfigDiff{
			ConfigCommon: cmd.ConfigCommon{
				LogLevel:       logLevel,
				LogFilename:    logfile,
				LocalRepoPath:  localRepo,
				KnownHostsPath: knownHosts,
			},
			GitRevision: diffGitRevision,
			ShowDiff:    diffShowDiff,
		}
		r, err := cmd.RunDiff(conf)
		printDiffResult(os.Stdout, r, diffShowDiff)
		if r != nil {
			printWarnings(os.Stderr, r.Warnings)
		}
		return err
	},
}

var (
	diffGitRevision string
	diffShowDiff    bool
)

func init() {
	rootCmd.AddCommand(diffCmd)
	diffCmd.PersistentFlags().StringVarP(&diffGitRevision, "revision", "b", "",
		"branch-name, tag-name, or commit-hash to compare with (default: the revision deployed last to each host)")
	diffCmd.PersistentFlags().BoolVar(&diffShowDiff, "show-diff", false,
		"show the content of differences")
}
