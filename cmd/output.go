package cmd

import (
	"fmt"
	"io"

	"github.com/ShotaKitazawa/isucontinuous/pkg/cmd"
)

func printWarnings(w io.Writer, warnings []string) {
	for _, msg := range warnings {
		fmt.Fprintf(w, "warning: %s\n", msg)
	}
}

func printDeployResult(w io.Writer, r *cmd.DeployResult) {
	if r == nil {
		return
	}
	if r.Hash != "" {
		fmt.Fprintf(w, "hash: %s\nmessage: %s\n\n", r.Hash, r.Message)
	}
}

func printPushResult(w io.Writer, r *cmd.PushResult) {
	for _, f := range r.Files {
		if f.Deleted {
			fmt.Fprintf(w, "committed: %s (deleted)\n", f.Path)
		} else {
			fmt.Fprintf(w, "committed: %s (%d bytes)\n", f.Path, f.Size)
		}
	}
}
