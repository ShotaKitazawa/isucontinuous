package cmd

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/ShotaKitazawa/isucontinuous/pkg/cmd"
	"github.com/ShotaKitazawa/isucontinuous/pkg/mcpserver"
)

// mcpCmd represents the mcp command
var mcpCmd = &cobra.Command{
	Use:   "mcp",
	Short: "Serve the commands as an MCP server on stdio",
	RunE: func(c *cobra.Command, args []string) error {
		limits, err := mcpLimits(c)
		if err != nil {
			return err
		}
		// executed stays false: the "output log" notice would go to stdout and break the protocol
		return mcpserver.Run(c.Context(), mcpserver.Options{
			Common: cmd.ConfigCommon{
				LogLevel:       logLevel,
				LogFilename:    logfile,
				LocalRepoPath:  localRepo,
				KnownHostsPath: knownHosts,
			},
			SlackToken: getenvDefault("slack-token", ""),
			Limits:     limits,
		})
	},
}

var (
	mcpMaxFileBytes     int
	mcpMaxResponseBytes int
)

func init() {
	rootCmd.AddCommand(mcpCmd)
	mcpCmd.Flags().IntVar(&mcpMaxFileBytes, "max-file-bytes", mcpserver.DefaultMaxFileBytes,
		"bytes of one output file of afterbench kept in a response (env MCP_MAX_FILE_BYTES)")
	mcpCmd.Flags().IntVar(&mcpMaxResponseBytes, "max-response-bytes", mcpserver.DefaultMaxResponseBytes,
		"bytes of all output files of afterbench kept in a response (env MCP_MAX_RESPONSE_BYTES)")
}

// mcpLimits lets a flag win over an environment variable, which wins over the default.
func mcpLimits(c *cobra.Command) (mcpserver.Limits, error) {
	resolve := func(flagName, env string, value int) (int, error) {
		if !c.Flags().Changed(flagName) {
			if v := getenvDefault(env, ""); v != "" {
				n, err := strconv.Atoi(v)
				if err != nil {
					return 0, fmt.Errorf("%s must be an integer: %w", strings.ToUpper(env), err)
				}
				value = n
			}
		}
		if value <= 0 {
			return 0, fmt.Errorf("%s must be positive", flagName)
		}
		return value, nil
	}
	file, err := resolve("max-file-bytes", "mcp-max-file-bytes", mcpMaxFileBytes)
	if err != nil {
		return mcpserver.Limits{}, err
	}
	response, err := resolve("max-response-bytes", "mcp-max-response-bytes", mcpMaxResponseBytes)
	if err != nil {
		return mcpserver.Limits{}, err
	}
	return mcpserver.Limits{MaxFileBytes: file, MaxResponseBytes: response}, nil
}
