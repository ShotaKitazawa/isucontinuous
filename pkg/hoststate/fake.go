package hoststate

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path"
	"strings"
	"sync"

	"github.com/ShotaKitazawa/isucontinuous/pkg/shell"
)

// FakeShell is an in-memory host that understands the commands of this package, so that callers can test
// lock handling without gomock expectations for each command. Other commands succeed with no output.
type FakeShell struct {
	mu    sync.Mutex
	name  string
	dirs  map[string]bool
	files map[string]string
	// PrimaryWritable false makes every command that writes under DirPrimary fail.
	PrimaryWritable bool
	// ExecError, if set, decides the result of commands this package does not know.
	ExecError func(command string) error
}

func NewFakeShell(name string) *FakeShell {
	return &FakeShell{name: name, dirs: map[string]bool{}, files: map[string]string{}, PrimaryWritable: true}
}

func (f *FakeShell) Host() string { return f.name }

func unquote(s string) string {
	s = strings.TrimSuffix(strings.TrimPrefix(s, "'"), "'")
	return strings.ReplaceAll(s, `'\''`, "'")
}

func (f *FakeShell) writable(p string) bool {
	return f.PrimaryWritable || !strings.HasPrefix(p, DirPrimary+"/") && p != DirPrimary
}

func fail(msg string) (bytes.Buffer, bytes.Buffer, error) {
	return bytes.Buffer{}, *bytes.NewBufferString(msg), fmt.Errorf("%s", msg)
}

func (f *FakeShell) Execf(ctx context.Context, basedir, command string, a ...interface{}) (bytes.Buffer, bytes.Buffer, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	arg := func(i int) string { return unquote(fmt.Sprint(a[i])) }
	switch command {
	case readLockFmt:
		for _, d := range []string{arg(0), arg(1)} {
			if f.dirs[path.Join(d, LockDirname)] {
				return *bytes.NewBufferString("locked " + d + "\n" + f.files[path.Join(d, LockDirname, LockInfoname)]), bytes.Buffer{}, nil
			}
		}
		return bytes.Buffer{}, bytes.Buffer{}, nil
	case createLockFmt:
		if !f.writable(arg(1)) {
			return fail("permission denied")
		}
		if f.dirs[arg(1)] {
			return fail("File exists")
		}
		f.dirs[arg(0)], f.dirs[arg(1)] = true, true
		return bytes.Buffer{}, bytes.Buffer{}, nil
	case writeInfoFmt:
		dir := path.Dir(arg(1))
		if !f.dirs[dir] || !f.writable(dir) {
			return fail("cannot write")
		}
		f.files[arg(1)] = arg(0) + "\n"
		return bytes.Buffer{}, bytes.Buffer{}, nil
	case removeLockFmt:
		if !f.writable(arg(0)) {
			return fail("permission denied")
		}
		delete(f.dirs, arg(0))
		delete(f.files, path.Join(arg(0), LockInfoname))
		return bytes.Buffer{}, bytes.Buffer{}, nil
	case mkdirAndWriteFmt:
		if !f.writable(arg(2)) {
			return fail("permission denied")
		}
		f.dirs[arg(0)] = true
		f.files[arg(2)] = fmt.Sprint(a[1]) + "\n"
		return bytes.Buffer{}, bytes.Buffer{}, nil
	case removeFileFmt:
		delete(f.files, arg(0))
		return bytes.Buffer{}, bytes.Buffer{}, nil
	case readLastFmt:
		for _, p := range []string{arg(0), arg(1)} {
			if c, ok := f.files[p]; ok {
				return *bytes.NewBufferString(c), bytes.Buffer{}, nil
			}
		}
		return bytes.Buffer{}, bytes.Buffer{}, nil
	}
	return f.unknown(fmt.Sprintf(command, a...))
}

func (f *FakeShell) unknown(command string) (bytes.Buffer, bytes.Buffer, error) {
	if f.ExecError != nil {
		if err := f.ExecError(command); err != nil {
			return bytes.Buffer{}, bytes.Buffer{}, err
		}
	}
	return bytes.Buffer{}, bytes.Buffer{}, nil
}

func (f *FakeShell) Exec(ctx context.Context, basedir, command string) (bytes.Buffer, bytes.Buffer, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.unknown(command)
}

func (f *FakeShell) Deploy(ctx context.Context, src, dst string) error { return nil }

func (f *FakeShell) ReadFile(ctx context.Context, p string) ([]byte, error) {
	return nil, os.ErrNotExist
}

// LockContent returns the info file of the lock in DirPrimary or DirFallback, for assertions.
func (f *FakeShell) LockContent() (string, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, d := range []string{DirPrimary, DirFallback} {
		if f.dirs[path.Join(d, LockDirname)] {
			return f.files[path.Join(d, LockDirname, LockInfoname)], true
		}
	}
	return "", false
}

var _ shell.Iface = (*FakeShell)(nil)
