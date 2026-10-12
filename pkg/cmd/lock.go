package cmd

import (
	"context"
	"sync"
	"time"

	"go.uber.org/zap"

	"github.com/ShotaKitazawa/isucontinuous/pkg/config"
	myerrors "github.com/ShotaKitazawa/isucontinuous/pkg/errors"
	"github.com/ShotaKitazawa/isucontinuous/pkg/hoststate"
	"github.com/ShotaKitazawa/isucontinuous/pkg/localrepo"
	"github.com/ShotaKitazawa/isucontinuous/pkg/usecases/lock"
)

// acquireAll takes the locks of all hosts or none: when any host fails, the locks created by this call are released.
// A lock the owner already held is never released here, because it may belong to a deploy that is still being measured,
// and its content is rewritten only after every host succeeded, so a failed call leaves it as it was.
func acquireAll(
	ctx context.Context, logger *zap.Logger, lockers map[string]*lock.Locker, hosts []config.Host,
	owner string, want hoststate.Lock, opts hoststate.AcquireOptions,
) (map[string]hoststate.AcquireOutcome, error) {
	refresh := opts.Refresh
	opts.Refresh = false
	outcomes := make([]hoststate.AcquireOutcome, len(hosts))
	errs := make([]error, len(hosts))
	var wg sync.WaitGroup
	for idx, host := range hosts {
		idx, host := idx, host
		wg.Add(1)
		go func() {
			defer wg.Done()
			outcomes[idx], errs[idx] = lockers[host.Name].Acquire(ctx, want, opts)
		}()
	}
	wg.Wait()
	var first error
	for idx, err := range errs {
		if err == nil {
			continue
		}
		logger.Error(err.Error(), zap.String("host", hosts[idx].Name))
		if first == nil {
			first = err
		}
	}
	result := make(map[string]hoststate.AcquireOutcome, len(hosts))
	for idx, host := range hosts {
		if errs[idx] == nil {
			result[host.Name] = outcomes[idx]
		}
	}
	if first != nil {
		for idx, host := range hosts {
			if errs[idx] == nil && outcomes[idx] != hoststate.Reentered {
				releaseOwn(ctx, logger, lockers[host.Name], owner, host.Name)
			}
		}
		return nil, first
	}
	if refresh {
		for idx, host := range hosts {
			if outcomes[idx] != hoststate.Reentered {
				continue
			}
			if _, err := lockers[host.Name].Acquire(ctx, want, hoststate.AcquireOptions{Refresh: true}); err != nil {
				return nil, err
			}
		}
	}
	return result, nil
}

// requireAll checks that the owner holds the lock of every host and returns the locks.
func requireAll(
	ctx context.Context, logger *zap.Logger, lockers map[string]*lock.Locker, hosts []config.Host, owner string,
) (map[string]hoststate.Lock, error) {
	locks := make([]hoststate.Lock, len(hosts))
	errs := make([]error, len(hosts))
	var wg sync.WaitGroup
	for idx, host := range hosts {
		idx, host := idx, host
		wg.Add(1)
		go func() {
			defer wg.Done()
			locks[idx], errs[idx] = lockers[host.Name].Require(ctx, owner)
		}()
	}
	wg.Wait()
	var first error
	result := make(map[string]hoststate.Lock, len(hosts))
	for idx, err := range errs {
		if err != nil {
			logger.Error(err.Error(), zap.String("host", hosts[idx].Name))
			if first == nil {
				first = err
			}
			continue
		}
		result[hosts[idx].Name] = locks[idx]
	}
	return result, first
}

// checkHeadMatchesLocks fails if HEAD is not the commit every host was locked with: sync, import or push
// may have moved local-repo since the deploy, and profiling would then run another yaml. A lock without a revision never matches.
func checkHeadMatchesLocks(ctx context.Context, repo localrepo.LocalRepoIface, hosts []config.Host, locks map[string]hoststate.Lock) error {
	head, _, err := repo.GetHeadInfo(ctx)
	if err != nil {
		return err
	}
	for _, host := range hosts {
		if locked := locks[host.Name].Revision; locked != head {
			return myerrors.NewErrorRevisionMismatch(host.Name, locked, head)
		}
	}
	return nil
}

// releaseOwn releases the lock of the owner; a failure is logged because the caller is already handling another result.
func releaseOwn(ctx context.Context, logger *zap.Logger, l *lock.Locker, owner, host string) {
	if _, _, err := l.Release(ctx, owner, false); err != nil {
		logger.Error("failed to release the lock: "+err.Error(), zap.String("host", host))
	}
}

func releaseAllOwn(ctx context.Context, logger *zap.Logger, lockers map[string]*lock.Locker, hosts []config.Host, owner string) {
	for _, host := range hosts {
		releaseOwn(ctx, logger, lockers[host.Name], owner, host.Name)
	}
}

// lockForCommand locks all hosts for the duration of a command such as setup or import. The returned function
// releases only the locks this call created, so that a lock held for a deploy survives the command.
func lockForCommand(
	ctx context.Context, logger *zap.Logger, conf ConfigCommon, repo localrepo.LocalRepoIface,
	newLockersFunc lock.NewLockersFunc, hosts []config.Host, operation string,
) (unlock func(), err error) {
	owner, err := conf.owner()
	if err != nil {
		return nil, err
	}
	lockers, err := newLockersFunc(logger, hosts)
	if err != nil {
		return nil, err
	}
	// the revision is informational here, so a repository without a commit does not block the command
	hash, _, err := repo.GetHeadInfo(ctx)
	if err != nil {
		hash = ""
	}
	want := hoststate.Lock{Owner: owner, Revision: hash, AcquiredAt: time.Now(), Operation: operation}
	outcomes, err := acquireAll(ctx, logger, lockers, hosts, owner, want, hoststate.AcquireOptions{})
	if err != nil {
		return nil, err
	}
	return func() {
		for _, host := range hosts {
			if outcomes[host.Name] != hoststate.Reentered {
				releaseOwn(ctx, logger, lockers[host.Name], owner, host.Name)
			}
		}
	}, nil
}
