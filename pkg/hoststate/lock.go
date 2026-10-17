package hoststate

import (
	"context"
	"fmt"
	"os"
	"os/user"
	"path"
	"strings"
	"time"

	"github.com/ShotaKitazawa/isucontinuous/pkg/shell"
)

const (
	LockDirname  = "lock"
	LockInfoname = "info"

	readLockFmt   = `for d in %s %s; do if [ -d "$d/lock" ]; then echo "locked $d"; cat "$d/lock/info" 2>/dev/null; exit 0; fi; done; true`
	createLockFmt = `mkdir -p %s && mkdir %s`
	writeInfoFmt  = `printf '%%s\n' %s > %s`
	removeLockFmt = `rm -rf %s`
)

// Lock is the content of the lock file of a host.
type Lock struct {
	Owner      string // user@host of the machine that runs isucontinuous
	Revision   string // commit hash
	AcquiredAt time.Time
	Operation  string
}

// LockedError is returned when the host is locked by someone else.
type LockedError struct {
	Host   string
	Holder Lock
}

func (e *LockedError) Error() string {
	return fmt.Sprintf("%s is locked by %s", e.Host, e.Holder.describe())
}

// NotLockedError is returned when the host is not locked by the caller.
type NotLockedError struct {
	Host string
}

func (e *NotLockedError) Error() string {
	return fmt.Sprintf("%s is not locked by you: run `deploy` first", e.Host)
}

func (l Lock) describe() string {
	if l.Owner == "" {
		return "unknown (the lock file has no content)"
	}
	return fmt.Sprintf("%s (operation: %s, revision: %.12s, since %s)",
		l.Owner, l.Operation, l.Revision, l.AcquiredAt.Format(time.RFC3339))
}

// Describe returns a one-line summary of the holder.
func (l Lock) Describe() string { return l.describe() }

// CurrentOwner returns user@host of this machine; the CLI and the MCP server share it.
func CurrentOwner() (string, error) {
	u, err := user.Current()
	if err != nil {
		return "", err
	}
	h, err := os.Hostname()
	if err != nil {
		return "", err
	}
	return u.Username + "@" + h, nil
}

func (l Lock) encode() (string, error) {
	for _, v := range []string{l.Owner, l.Revision, l.Operation} {
		if strings.ContainsAny(v, "\r\n") {
			return "", fmt.Errorf("invalid lock value: %q", v)
		}
	}
	if l.Owner == "" {
		return "", fmt.Errorf("lock owner is empty")
	}
	if l.Revision != "" && !commitHashPattern.MatchString(l.Revision) {
		return "", fmt.Errorf("invalid commit hash: %q", l.Revision)
	}
	return fmt.Sprintf("owner=%s\nrevision=%s\nacquired_at=%s\noperation=%s",
		l.Owner, l.Revision, l.AcquiredAt.UTC().Format(time.RFC3339), l.Operation), nil
}

func decodeLock(text string) Lock {
	var l Lock
	for _, line := range strings.Split(text, "\n") {
		kv := strings.SplitN(strings.TrimSpace(line), "=", 2)
		if len(kv) != 2 {
			continue
		}
		switch kv[0] {
		case "owner":
			l.Owner = kv[1]
		case "revision":
			l.Revision = kv[1]
		case "acquired_at":
			l.AcquiredAt, _ = time.Parse(time.RFC3339, kv[1])
		case "operation":
			l.Operation = kv[1]
		}
	}
	return l
}

// readLock finds the lock in either directory. dir is the directory that contains the lock directory.
func readLock(ctx context.Context, sh shell.Iface) (lock Lock, dir string, found bool, err error) {
	stdout, _, err := sh.Execf(ctx, "", readLockFmt, shell.Quote(DirPrimary), shell.Quote(DirFallback))
	if err != nil {
		return Lock{}, "", false, err
	}
	head, rest := stdout.String(), ""
	if i := strings.Index(head, "\n"); i >= 0 {
		head, rest = head[:i], head[i+1:]
	}
	switch strings.TrimPrefix(head, "locked ") {
	case DirPrimary:
		dir = DirPrimary
	case DirFallback:
		dir = DirFallback
	default:
		return Lock{}, "", false, nil
	}
	return decodeLock(rest), dir, true, nil
}

// ReadLock returns the lock of the host; found is false if the host is not locked.
func ReadLock(ctx context.Context, sh shell.Iface) (lock Lock, found bool, err error) {
	lock, _, found, err = readLock(ctx, sh)
	return lock, found, err
}

type AcquireOptions struct {
	// Force takes the lock over from another owner.
	Force bool
	// Refresh rewrites the lock content when the caller already holds the lock.
	Refresh bool
}

type AcquireOutcome int

const (
	// Created means the lock did not exist before.
	Created AcquireOutcome = iota + 1
	// Reentered means the caller already held the lock.
	Reentered
	// TookOver means the lock of another owner was removed by Force.
	TookOver
)

func writeInfo(ctx context.Context, sh shell.Iface, dir string, want Lock) error {
	content, err := want.encode()
	if err != nil {
		return err
	}
	infoPath := path.Join(dir, LockDirname, LockInfoname)
	if _, stderr, err := sh.Execf(ctx, "", writeInfoFmt, shell.Quote(content), shell.Quote(infoPath)); err != nil {
		return fmt.Errorf("failed to write the lock: %s", strings.TrimSpace(stderr.String()))
	}
	return nil
}

func removeLock(ctx context.Context, sh shell.Iface, dir string) error {
	if _, stderr, err := sh.Execf(ctx, "", removeLockFmt, shell.Quote(path.Join(dir, LockDirname))); err != nil {
		return fmt.Errorf("failed to remove the lock: %s", strings.TrimSpace(stderr.String()))
	}
	return nil
}

// AcquireLock takes the lock of the host without waiting. mkdir is atomic, so of two concurrent callers only one succeeds.
func AcquireLock(ctx context.Context, sh shell.Iface, want Lock, opts AcquireOptions) (AcquireOutcome, error) {
	if _, err := want.encode(); err != nil {
		return 0, err
	}
	outcome := Created
	cur, dir, found, err := readLock(ctx, sh)
	if err != nil {
		return 0, err
	}
	if found {
		switch {
		case cur.Owner == want.Owner:
			if opts.Refresh {
				if err := writeInfo(ctx, sh, dir, want); err != nil {
					return 0, err
				}
			}
			return Reentered, nil
		case !opts.Force:
			return 0, &LockedError{Host: sh.Host(), Holder: cur}
		}
		if err := removeLock(ctx, sh, dir); err != nil {
			return 0, err
		}
		outcome = TookOver
	}
	var lastErr error
	for _, d := range []string{DirPrimary, DirFallback} {
		_, stderr, err := sh.Execf(ctx, "", createLockFmt, shell.Quote(d), shell.Quote(path.Join(d, LockDirname)))
		if err != nil {
			lastErr = fmt.Errorf("failed to create the lock: %s", strings.TrimSpace(stderr.String()))
			// someone may have created it between the read and the mkdir
			if cur, _, found, rerr := readLock(ctx, sh); rerr == nil && found {
				if cur.Owner == want.Owner {
					return Reentered, nil
				}
				return 0, &LockedError{Host: sh.Host(), Holder: cur}
			}
			continue
		}
		if err := writeInfo(ctx, sh, d, want); err != nil {
			_ = removeLock(ctx, sh, d)
			return 0, err
		}
		return outcome, nil
	}
	return 0, lastErr
}

type ReleaseOutcome int

const (
	// NotLocked means there was no lock.
	NotLocked ReleaseOutcome = iota + 1
	Released
	// HeldByOther means the lock is owned by someone else and force was not given.
	HeldByOther
)

// ReleaseLock removes the lock if owner holds it, or whoever holds it when force is true.
// holder is the lock that was found, if any.
func ReleaseLock(ctx context.Context, sh shell.Iface, owner string, force bool) (outcome ReleaseOutcome, holder *Lock, err error) {
	cur, dir, found, err := readLock(ctx, sh)
	if err != nil {
		return 0, nil, err
	}
	if !found {
		return NotLocked, nil, nil
	}
	if cur.Owner != owner && !force {
		return HeldByOther, &cur, nil
	}
	if err := removeLock(ctx, sh, dir); err != nil {
		return 0, &cur, err
	}
	return Released, &cur, nil
}

// RequireLock returns the lock if owner holds it.
func RequireLock(ctx context.Context, sh shell.Iface, owner string) (Lock, error) {
	cur, found, err := ReadLock(ctx, sh)
	if err != nil {
		return Lock{}, err
	}
	if !found {
		return Lock{}, &NotLockedError{Host: sh.Host()}
	}
	if cur.Owner != owner {
		return Lock{}, &LockedError{Host: sh.Host(), Holder: cur}
	}
	return cur, nil
}
