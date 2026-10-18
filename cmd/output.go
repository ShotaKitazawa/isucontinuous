package cmd

import (
	"fmt"
	"io"
	"time"

	"github.com/ShotaKitazawa/isucontinuous/pkg/cmd"
	"github.com/ShotaKitazawa/isucontinuous/pkg/localrepo"
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
		fmt.Fprintf(w, "hash: %s\ndate: %s\nmessage: %s\n\n", r.Hash, r.Date.Format(time.RFC3339), r.Message)
	}
}

func printPushedFiles(w io.Writer, files []localrepo.PushedFile) {
	for _, f := range files {
		if f.Deleted {
			fmt.Fprintf(w, "committed: %s (deleted)\n", f.Path)
		} else {
			fmt.Fprintf(w, "committed: %s (%d bytes)\n", f.Path, f.Size)
		}
	}
}

func printDiffResult(w io.Writer, r *cmd.DiffResult, showDiff bool) {
	if r == nil {
		return
	}
	for _, h := range r.Diffs {
		switch {
		case h.NoRecord:
			fmt.Fprintf(w, "%s: no record of the last deployed revision, nothing to compare\n", h.Host)
			continue
		case len(h.Changes) == 0:
			fmt.Fprintf(w, "%s: no difference from %.12s\n", h.Host, h.Revision)
		default:
			fmt.Fprintf(w, "%s: %d file(s) differ from %.12s\n", h.Host, len(h.Changes), h.Revision)
		}
		for _, c := range h.Changes {
			fmt.Fprintf(w, "  %s: %s\n", c.Kind, c.Path)
			if showDiff {
				fmt.Fprint(w, c.Diff)
			}
		}
		for _, p := range h.Skipped {
			fmt.Fprintf(w, "  skipped (not in the revision): %s\n", p)
		}
	}
}

func printReleaseResult(w io.Writer, r *cmd.ReleaseResult) {
	if r == nil {
		return
	}
	for _, h := range r.Releases {
		if h.State == cmd.ReleaseHeldByOther {
			fmt.Fprintf(w, "%s: %s: %s (use --force to release)\n", h.Host, h.State, h.Holder.Describe())
			continue
		}
		fmt.Fprintf(w, "%s: %s\n", h.Host, h.State)
	}
}

func printStatusResult(w io.Writer, r *cmd.StatusResult) {
	if r == nil {
		return
	}
	for _, h := range r.Statuses {
		if h.Disabled {
			fmt.Fprintf(w, "%s: disabled\n", h.Host)
			continue
		}
		lock := "unlocked"
		if h.Lock != nil {
			lock = "locked by " + h.Lock.Describe()
		}
		last := "none recorded"
		if h.LastRevision != "" {
			last = fmt.Sprintf("%.12s", h.LastRevision)
		}
		fmt.Fprintf(w, "%s: %s; last deployed revision: %s\n", h.Host, lock, last)
	}
}
