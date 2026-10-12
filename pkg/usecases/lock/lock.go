package lock

import (
	"context"

	"go.uber.org/zap"
	"k8s.io/utils/exec"

	"github.com/ShotaKitazawa/isucontinuous/pkg/config"
	"github.com/ShotaKitazawa/isucontinuous/pkg/hoststate"
	"github.com/ShotaKitazawa/isucontinuous/pkg/shell"
)

type Locker struct {
	log   *zap.Logger
	shell shell.Iface
}

// NewLockersFunc returns lockers keyed by host name.
type NewLockersFunc func(*zap.Logger, []config.Host) (map[string]*Locker, error)

func NewLockers(logger *zap.Logger, hosts []config.Host) (map[string]*Locker, error) {
	lockers := make(map[string]*Locker)
	for _, host := range hosts {
		var s shell.Iface
		if host.Local {
			s = shell.NewLocalClient(exec.New())
		} else {
			var err error
			s, err = shell.NewSshClient(host.Host, host.Port, host.User, host.Password, host.Key)
			if err != nil {
				return nil, err
			}
		}
		lockers[host.Name] = NewWithShell(logger, s)
	}
	return lockers, nil
}

// NewWithShell builds a Locker on an existing shell, which lets callers substitute a fake host.
func NewWithShell(logger *zap.Logger, s shell.Iface) *Locker {
	return &Locker{logger, s}
}

func (l Locker) Acquire(ctx context.Context, want hoststate.Lock, opts hoststate.AcquireOptions) (hoststate.AcquireOutcome, error) {
	return hoststate.AcquireLock(ctx, l.shell, want, opts)
}

func (l Locker) Release(ctx context.Context, owner string, force bool) (hoststate.ReleaseOutcome, *hoststate.Lock, error) {
	return hoststate.ReleaseLock(ctx, l.shell, owner, force)
}

func (l Locker) Require(ctx context.Context, owner string) (hoststate.Lock, error) {
	return hoststate.RequireLock(ctx, l.shell, owner)
}

// Status returns the holder (nil if unlocked) and the revision deployed last (empty if none is recorded).
func (l Locker) Status(ctx context.Context) (holder *hoststate.Lock, lastRevision string, err error) {
	lock, found, err := hoststate.ReadLock(ctx, l.shell)
	if err != nil {
		return nil, "", err
	}
	if found {
		holder = &lock
	}
	lastRevision, _, err = hoststate.ReadLastRevision(ctx, l.shell)
	return holder, lastRevision, err
}
