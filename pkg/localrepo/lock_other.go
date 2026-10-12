//go:build !(darwin || dragonfly || freebsd || linux || netbsd || openbsd)
// +build !darwin,!dragonfly,!freebsd,!linux,!netbsd,!openbsd

package localrepo

// Windows is out of scope, so there is no emulation of flock here.
func lockFile(path string) (func(), error) {
	return func() {}, nil
}
