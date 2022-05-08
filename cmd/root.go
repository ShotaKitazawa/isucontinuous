package cmd

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
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

// utils

func getenvDefault(flagName, defaultV string) string {
	key := strings.ToUpper(strings.ReplaceAll(flagName, "-", "_"))
	result := os.Getenv(key)
	if result == "" {
		return defaultV
	}
	return result
}

const requiredFlagAnnotation = "isucontinuous/required"

func setRequired(cmd *cobra.Command, flagNames ...string) {
	for _, flagName := range flagNames {
		if err := cmd.PersistentFlags().SetAnnotation(flagName, requiredFlagAnnotation, []string{"true"}); err != nil {
			log.Fatal(err)
		}
	}
}

func checkRequiredFlags(flags *pflag.FlagSet) error {
	requiredError := false
	flagName := ""
	flags.VisitAll(func(flag *pflag.Flag) {
		requiredAnnotation := flag.Annotations[requiredFlagAnnotation]
		if len(requiredAnnotation) == 0 {
			return
		}
		flagRequired := requiredAnnotation[0] == "true"
		if flagRequired && !flag.Changed && getenvDefault(flag.Name, "") == "" {
			requiredError = true
			flagName = flag.Name
		}
	})
	if requiredError {
		return errors.New("Required flag `" + flagName + "` has not been set")
	}
	return nil
}
