package cmd

import (
	"context"
	"os"
	"os/signal"

	"github.com/spf13/cobra"

	"github.com/ShotaKitazawa/isucontinuous/pkg/cmd"
	"github.com/ShotaKitazawa/isucontinuous/pkg/mcpserver"
)

// mcpCmd represents the mcp command
var mcpCmd = &cobra.Command{
	Use:   "mcp",
	Short: "Serve the commands as an MCP server on stdio",
	RunE: func(c *cobra.Command, args []string) error {
		// executed stays false: the "output log" notice would go to stdout and break the protocol
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
		defer stop()
		return mcpserver.Run(ctx, mcpserver.Options{
			Common: cmd.ConfigCommon{
				LogLevel:       logLevel,
				LogFilename:    logfile,
				LocalRepoPath:  localRepo,
				KnownHostsPath: knownHosts,
			},
			SlackToken: getenvDefault("slack-token", ""),
		})
	},
}

func init() {
	rootCmd.AddCommand(mcpCmd)
}
