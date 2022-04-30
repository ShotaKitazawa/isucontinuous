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
	Use:          "isu-continuous",
	SilenceUsage: true,
	Short:        "isu-continuous is tool to support Continuous Deployment, Benchmark, and Profiling!",
}

var executed bool

func Execute() {
	err := rootCmd.Execute()
	if executed {
		fmt.Printf("=> output log to %s\n", logfile)
	}
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
	rootCmd.PersistentFlags().StringVarP(&logLevel, "log-level", "v", "INFO",
		"log-level (DEBUG, INFO, or ERROR)")
	rootCmd.PersistentFlags().StringVarP(&logfile, "logfile", "o", filepath.Join(os.Getenv("HOME"), "isucontinuous.log"),
		"path of log file")
	rootCmd.PersistentFlags().StringVarP(&localRepo, "local-repo", "l", filepath.Join(os.Getenv("HOME"), "local-repo"),
		"local repository's path managed by isu-continuous")
}
