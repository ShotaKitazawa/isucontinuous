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
	Short:        "isucontinuous is tool to support Continuous Deployment, Benchmark, and Profiling!",
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
	rootCmd.PersistentFlags().StringVarP(&logLevel, "log-level", "v", getenvDefault("LOG_LEVEL", "INFO"),
		"log-level (DEBUG, INFO, or ERROR)")
	defaultLogfile := filepath.Join(os.Getenv("HOME"), "isucontinuous.log")
	rootCmd.PersistentFlags().StringVarP(&logfile, "logfile", "o", getenvDefault("LOGFILE", defaultLogfile),
		"path of log file")
	defaultLocalRepo := filepath.Join(os.Getenv("HOME"), "local-repo")
	rootCmd.PersistentFlags().StringVarP(&localRepo, "local-repo", "l", getenvDefault("LOCAL_REPO", defaultLocalRepo),
		"local repository's path managed by isucontinuous")
}

func getenvDefault(key, defaultV string) string {
	result := os.Getenv(key)
	if result == "" {
		return defaultV
	}
	return result
}
