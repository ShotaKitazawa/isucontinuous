package cmd

import (
	"os"

	"github.com/spf13/cobra"

	"github.com/ShotaKitazawa/isucontinuous/pkg/cmd"
)

// importCmd represents the import command
var importCmd = &cobra.Command{
	Use:   "import",
	Short: "Import some files from hosts[].deploy.files[].target",
	PreRunE: func(cmd *cobra.Command, args []string) error {
		return checkRequiredFlags(cmd.Flags())
	},
	RunE: func(c *cobra.Command, args []string) error {
		executed = true
		conf := cmd.ConfigImport{
			ConfigCommon: cmd.ConfigCommon{
				LogLevel:       logLevel,
				LogFilename:    logfile,
				LocalRepoPath:  localRepo,
				KnownHostsPath: knownHosts,
			},
			GitBranch: importGitBranch,
			Message:   importMessage,
		}
		r, err := cmd.RunImportContext(c.Context(), conf)
		if r != nil {
			printPushResult(os.Stdout, &cmd.PushResult{Files: r.Files})
			printWarnings(os.Stderr, r.Warnings)
		}
		return err
	},
}

var (
	importGitBranch string
	importMessage   string
)

func init() {
	rootCmd.AddCommand(importCmd)
	importCmd.PersistentFlags().StringVarP(&importGitBranch, "branch", "b", "master",
		"branch-name to synchronize and push to Git remote-repo")
	importCmd.PersistentFlags().StringVarP(&importMessage, "message", "m", "",
		"commit message (default: \"commit by isucontinuous\")")
}
