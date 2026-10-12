// Package hoststate reads and writes state files that isucontinuous keeps on each host.
package hoststate

import (
	"context"
	"fmt"
	"path"
	"regexp"
	"strings"

	"github.com/ShotaKitazawa/isucontinuous/pkg/shell"
)

const (
	// DirPrimary is preferred; DirFallback is used when the login user cannot write to DirPrimary.
	DirPrimary  = "/var/lib/isucontinuous"
	DirFallback = "/tmp/isucontinuous"

	LastRevisionFilename = "last-revision"
)

var commitHashPattern = regexp.MustCompile(`^[0-9a-f]{7,64}$`)

func lastRevisionPaths() (primary, fallback string) {
	return path.Join(DirPrimary, LastRevisionFilename), path.Join(DirFallback, LastRevisionFilename)
}

// WriteLastRevision stores the commit hash of the revision that was deployed last.
func WriteLastRevision(ctx context.Context, sh shell.Iface, hash string) error {
	if !commitHashPattern.MatchString(hash) {
		return fmt.Errorf("invalid commit hash: %q", hash)
	}
	primary, fallback := lastRevisionPaths()
	if _, _, err := sh.Execf(ctx, "", `mkdir -p %s && printf '%%s\n' %s > %s`,
		shell.Quote(DirPrimary), hash, shell.Quote(primary)); err == nil {
		_, _, _ = sh.Execf(ctx, "", `rm -f %s`, shell.Quote(fallback))
		return nil
	}
	if _, stderr, err := sh.Execf(ctx, "", `mkdir -p %s && printf '%%s\n' %s > %s`,
		shell.Quote(DirFallback), hash, shell.Quote(fallback)); err != nil {
		return fmt.Errorf("failed to record the last deployed revision: %s", strings.TrimSpace(stderr.String()))
	}
	return nil
}

// ReadLastRevision returns the commit hash recorded by WriteLastRevision; found is false if nothing is recorded.
func ReadLastRevision(ctx context.Context, sh shell.Iface) (hash string, found bool, err error) {
	primary, fallback := lastRevisionPaths()
	stdout, _, err := sh.Execf(ctx, "", `cat %s 2>/dev/null || cat %s 2>/dev/null || true`,
		shell.Quote(primary), shell.Quote(fallback))
	if err != nil {
		return "", false, err
	}
	hash = strings.TrimSpace(stdout.String())
	if hash == "" {
		return "", false, nil
	}
	if !commitHashPattern.MatchString(hash) {
		return "", false, fmt.Errorf("invalid content in the last-revision file: %q", hash)
	}
	return hash, true, nil
}
