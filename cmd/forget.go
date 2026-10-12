package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/ShotaKitazawa/isucontinuous/pkg/shell"
)

// forgetCmd is CLI-only: exposing it through pkg/cmd would give MCP a way around host key mismatches.
var forgetCmd = &cobra.Command{
	Use:   "forget <host[:port]>",
	Short: "Remove a host from the known_hosts file of isucontinuous",
	Args:  cobra.ExactArgs(1),
	RunE: func(c *cobra.Command, args []string) error {
		n, err := shell.ForgetHost(knownHosts, args[0])
		if err != nil {
			return err
		}
		fmt.Printf("removed %d entries of %s from %s\n", n, args[0], knownHosts)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(forgetCmd)
}
