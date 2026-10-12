//go:build darwin || dragonfly || freebsd || linux || netbsd || openbsd
// +build darwin dragonfly freebsd linux netbsd openbsd

package localrepo

import (
	"os"
	"syscall"

	myerrors "github.com/ShotaKitazawa/isucontinuous/pkg/errors"
)

// flock is released by the kernel when the process dies, which a lock file with a PID would not give.
func lockFile(path string) (func(), error) {
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0600)
	if err != nil {
		return nil, err
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		f.Close()
		if err == syscall.EWOULDBLOCK {
			return nil, myerrors.NewErrorLocalRepoLocked(path)
		}
		return nil, err
	}
	return func() { f.Close() }, nil
}
