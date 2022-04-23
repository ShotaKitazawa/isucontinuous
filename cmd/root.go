package cmd

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

// RootCmd is reviewappctl root CLI command.
var rootCmd = &cobra.Command{
	Use:          "isucontinuous",
	SilenceUsage: true,
	Short:        "isucontinuous is Continuous Deployment, Benchmark, and Profiling tool!",
	PostRun: func(cmd *cobra.Command, args []string) {
		fmt.Printf("output log to %s\n", logfile)
	},
}

// Execute executes the root command.
func Execute() {
	if err := getRootCmd(os.Args[1:]).Execute(); err != nil {
		os.Exit(1)
	}
}

var (
	logLevel  string
	logfile   string
	localRepo string
)

func getRootCmd(args []string) *cobra.Command {
	rootCmd.SetArgs(args)
	rootCmd.PersistentFlags().AddGoFlagSet(flag.CommandLine)
	rootCmd.PersistentFlags().StringVar(&logLevel, "log-level", "INFO",
		"log-level (DEBUG, INFO, or ERROR)")
	rootCmd.PersistentFlags().StringVarP(&logfile, "logfile", "l", "/var/log/isucontinuous.log",
		"path of log file")
	rootCmd.PersistentFlags().StringVarP(&localRepo, "local-repo", "r", filepath.Join(os.Getenv("HOME"), "isucontinuous"),
		"local repository's path managed by isucontinuous")

	return rootCmd
}
