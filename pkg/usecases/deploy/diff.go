package deploy

import (
	"bytes"
	"context"
	"fmt"

	"github.com/ShotaKitazawa/isucontinuous/pkg/config"
	"github.com/ShotaKitazawa/isucontinuous/pkg/hoststate"
)

// RevisionReader gives the content of a file at a commit, without touching the working tree.
type RevisionReader interface {
	ShowFile(ctx context.Context, hash, path string) ([]byte, bool, error)
}

type ChangeKind string

const (
	ChangeModified ChangeKind = "modified" // the host's content differs from the revision's
	ChangeMissing  ChangeKind = "missing"  // the host does not have the file
)

type FileChange struct {
	Path string // destination on the host
	Kind ChangeKind
	Diff string // filled only if requested; from the host's content to the revision's
}

type HostDiff struct {
	Host     string
	Revision string // commit hash compared against
	NoRecord bool   // no last deployed revision is recorded, so nothing was compared
	Changes  []FileChange
	Skipped  []string // files of the plan that the revision does not have
}

// LastRevision returns the commit hash that was deployed to the host last.
func (d Deployer) LastRevision(ctx context.Context) (string, bool, error) {
	return hoststate.ReadLastRevision(ctx, d.shell)
}

// Diff compares the content on the host with the files of the plan at the commit hash.
// File modes are not compared because Deploy never sets them, and files only on the host are ignored.
func (d Deployer) Diff(
	ctx context.Context, host string, targets []config.DeployTarget,
	hash string, reader RevisionReader, showDiff bool,
) (*HostDiff, error) {
	plan, err := d.Plan(ctx, host, targets)
	if err != nil {
		return nil, err
	}
	result := &HostDiff{Host: host, Revision: hash}
	var files []PlanFile
	want := make(map[string][]byte)
	for _, f := range plan.files() {
		content, found, err := reader.ShowFile(ctx, hash, f.RepoPath)
		if err != nil {
			return nil, err
		}
		if !found {
			result.Skipped = append(result.Skipped, f.RepoPath)
			continue
		}
		files = append(files, f)
		want[f.Dst] = content
	}
	dsts := make([]string, len(files))
	for i, f := range files {
		dsts[i] = f.Dst
	}
	sums, hashed, err := d.remoteSums(ctx, dsts)
	if err != nil {
		return nil, err
	}
	for _, f := range files {
		if remote, found := sums[f.Dst]; hashed && found && remote == sha256Hex(want[f.Dst]) {
			continue
		}
		// without sha256sum, or for names it cannot report, the content is compared locally
		current, err := d.shell.ReadFile(ctx, f.Dst)
		if isNotExist(err) {
			result.Changes = append(result.Changes, FileChange{Path: f.Dst, Kind: ChangeMissing})
			continue
		}
		if err != nil {
			return nil, err
		}
		if bytes.Equal(current, want[f.Dst]) {
			continue
		}
		change := FileChange{Path: f.Dst, Kind: ChangeModified}
		if showDiff {
			change.Diff = unifiedDiff(
				fmt.Sprintf("%s:%s", d.shell.Host(), f.Dst), fmt.Sprintf("%.12s:%s", hash, f.RepoPath),
				current, want[f.Dst])
		}
		result.Changes = append(result.Changes, change)
	}
	return result, nil
}
