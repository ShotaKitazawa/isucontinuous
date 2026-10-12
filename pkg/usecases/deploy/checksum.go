package deploy

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"strings"

	"github.com/ShotaKitazawa/isucontinuous/pkg/shell"
)

const sumChunkSize = 100

func sha256Hex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// remoteSums returns the sha256 of each existing file on the host. Files that are absent, or whose
// names sha256sum would escape, are left out. ok is false when the host has no sha256sum.
func (d Deployer) remoteSums(ctx context.Context, paths []string) (sums map[string]string, ok bool, err error) {
	if _, _, err := d.shell.Exec(ctx, "", "command -v sha256sum >/dev/null 2>&1"); err != nil {
		return nil, false, nil
	}
	sums = make(map[string]string)
	var queried []string
	for _, p := range paths {
		if !strings.ContainsAny(p, "\\\n") {
			queried = append(queried, p)
		}
	}
	for start := 0; start < len(queried); start += sumChunkSize {
		end := start + sumChunkSize
		if end > len(queried) {
			end = len(queried)
		}
		quoted := make([]string, 0, end-start)
		for _, p := range queried[start:end] {
			quoted = append(quoted, shell.Quote(p))
		}
		// missing files make sha256sum exit non-zero, which is not an error here
		stdout, _, err := d.shell.Execf(ctx, "", "sha256sum -- %s 2>/dev/null || true", strings.Join(quoted, " "))
		if err != nil {
			return nil, false, err
		}
		for _, line := range strings.Split(stdout.String(), "\n") {
			if len(line) < 66 || line[64] != ' ' {
				continue
			}
			sums[line[66:]] = line[:64]
		}
	}
	return sums, true, nil
}

// changedFiles returns the files whose content differs from the host's, or that the host lacks.
// Without sha256sum on the host every file is considered changed.
func (d Deployer) changedFiles(ctx context.Context, files []PlanFile) ([]PlanFile, error) {
	if len(files) == 0 {
		return nil, nil
	}
	dsts := make([]string, len(files))
	for i, f := range files {
		dsts[i] = f.Dst
	}
	sums, ok, err := d.remoteSums(ctx, dsts)
	if err != nil {
		return nil, err
	}
	if !ok {
		return files, nil
	}
	var changed []PlanFile
	for _, f := range files {
		b, err := os.ReadFile(f.Src)
		if err != nil {
			return nil, err
		}
		if remote, found := sums[f.Dst]; found && remote == sha256Hex(b) {
			continue
		}
		changed = append(changed, f)
	}
	return changed, nil
}

func isNotExist(err error) bool { return errors.Is(err, os.ErrNotExist) }
