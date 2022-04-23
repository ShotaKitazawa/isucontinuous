package cmd

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:          "isucontinuous",
	SilenceUsage: true,
	Short:        "isucontinuous is Continuous Deployment, Benchmark, and Profiling tool!",
}

func Execute() {
	err := rootCmd.Execute()
	fmt.Printf("=> output log to %s\n", logfile)
	if err != nil {
		os.Exit(1)
	}
}

var (
	logLevel  string
	logfile   string
	localRepo string
)

func init() {
	rootCmd.PersistentFlags().AddGoFlagSet(flag.CommandLine)
	rootCmd.PersistentFlags().StringVar(&logLevel, "log-level", "INFO",
		"log-level (DEBUG, INFO, or ERROR)")
	rootCmd.PersistentFlags().StringVarP(&logfile, "logfile", "l", "/var/log/isucontinuous.log",
		"path of log file")
	rootCmd.PersistentFlags().StringVarP(&localRepo, "local-repo", "r", filepath.Join(os.Getenv("HOME"), "isucontinuous"),
		"local repository's path managed by isucontinuous")
}
