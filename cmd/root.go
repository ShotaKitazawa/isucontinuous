package cmd

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

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
	rootCmd.PersistentFlags().StringVarP(&logLevel, "log-level", "v", "INFO",
		"log-level (DEBUG, INFO, or ERROR)")
	refStringEnvVarP(&logLevel, "log-level")
	rootCmd.PersistentFlags().StringVarP(&logfile, "logfile", "o", filepath.Join(os.Getenv("HOME"), "isucontinuous.log"),
		"path of log file")
	refStringEnvVarP(&logfile, "logfile")
	rootCmd.PersistentFlags().StringVarP(&localRepo, "local-repo", "l", filepath.Join(os.Getenv("HOME"), "local-repo"),
		"local repository's path managed by isucontinuous")
	refStringEnvVarP(&localRepo, "local-repo")
}

func refStringEnvVarP(p *string, name string) {
	if *p == "" {
		*p = os.Getenv(strings.ToUpper(strings.ReplaceAll(name, "-", "_")))
	}
}

func requiredFlag(p *string, name string) {
	if *p == "" {
		fmt.Printf(`option "%s" must not be empty`, name)
		os.Exit(1)
	}
}
