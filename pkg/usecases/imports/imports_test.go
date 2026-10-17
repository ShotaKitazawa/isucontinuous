package imports

import (
	"context"
	"os"
	osexec "os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"go.uber.org/zap/zaptest"
	kexec "k8s.io/utils/exec"

	"github.com/ShotaKitazawa/isucontinuous/pkg/shell"
)

func TestParseFileMode(t *testing.T) {
	tests := []struct {
		in      string
		want    os.FileMode
		wantErr bool
	}{
		{in: "644", want: 0644},
		{in: "755\n", want: 0755},
		{in: "600", want: 0600},
		{in: "0", want: 0},
		{in: "abc", wantErr: true},
		{in: "888", wantErr: true},
		{in: "", wantErr: true},
	}
	for _, tt := range tests {
		got, err := parseFileMode(tt.in)
		if (err != nil) != tt.wantErr {
			t.Errorf("parseFileMode(%q) error = %v, wantErr %v", tt.in, err, tt.wantErr)
			continue
		}
		if got != tt.want {
			t.Errorf("parseFileMode(%q) = %o, want %o", tt.in, got, tt.want)
		}
	}
}

func newTestImporter(t *testing.T) *Importer {
	t.Helper()
	if _, err := osexec.LookPath("git"); err != nil {
		t.Skip("git is not installed")
	}
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("GIT_CONFIG_GLOBAL", filepath.Join(home, ".gitconfig"))
	return new(zaptest.NewLogger(t), shell.NewLocalClient(kexec.New()))
}

func gitIn(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := osexec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@example.com",
		"GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@example.com")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v: %s", args, err, out)
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

func sorted(s []string) []string {
	sort.Strings(s)
	return s
}

func TestImporter_ListUntrackedFiles(t *testing.T) {
	ctx := context.Background()

	t.Run("not a repository", func(t *testing.T) {
		imp := newTestImporter(t)
		dir := t.TempDir()
		writeFile(t, filepath.Join(dir, "a.conf"), "a")
		writeFile(t, filepath.Join(dir, "sub", "b.conf"), "b")
		writeFile(t, filepath.Join(dir, "debug.log"), "x")
		writeFile(t, filepath.Join(dir, ".gitignore"), "*.log\n")
		files, err := imp.ListUntrackedFiles(ctx, dir)
		if err != nil {
			t.Fatal(err)
		}
		want := []string{".gitignore", "a.conf", "sub/b.conf"}
		if !reflect.DeepEqual(sorted(files), want) {
			t.Errorf("files = %v, want %v", files, want)
		}
		if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
			t.Error(".git must not be created in the target")
		}
	})

	t.Run("existing repository is kept", func(t *testing.T) {
		imp := newTestImporter(t)
		dir := t.TempDir()
		gitIn(t, dir, "init")
		writeFile(t, filepath.Join(dir, "tracked"), "t")
		gitIn(t, dir, "add", "-A")
		gitIn(t, dir, "commit", "-m", "init")
		writeFile(t, filepath.Join(dir, "untracked"), "u")
		files, err := imp.ListUntrackedFiles(ctx, dir)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(files, []string{"untracked"}) {
			t.Errorf("files = %v", files)
		}
		gitIn(t, dir, "log", "-1")
	})

	t.Run("parent gitignore is respected", func(t *testing.T) {
		imp := newTestImporter(t)
		parent := t.TempDir()
		gitIn(t, parent, "init")
		writeFile(t, filepath.Join(parent, ".gitignore"), "*.log\n")
		sub := filepath.Join(parent, "sub")
		writeFile(t, filepath.Join(sub, "a.conf"), "a")
		writeFile(t, filepath.Join(sub, "debug.log"), "x")
		files, err := imp.ListUntrackedFiles(ctx, sub)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(files, []string{"a.conf"}) {
			t.Errorf("files = %v, want [a.conf]", files)
		}
		if _, err := os.Stat(filepath.Join(sub, ".git")); err == nil {
			t.Error(".git must not be created in the target")
		}
	})

	t.Run("empty directory", func(t *testing.T) {
		imp := newTestImporter(t)
		files, err := imp.ListUntrackedFiles(ctx, t.TempDir())
		if err != nil {
			t.Fatal(err)
		}
		if len(files) != 0 {
			t.Errorf("files = %v, want none", files)
		}
	})

	t.Run("safe.directory is not duplicated", func(t *testing.T) {
		imp := newTestImporter(t)
		dir := t.TempDir()
		writeFile(t, filepath.Join(dir, "a"), "a")
		for i := 0; i < 2; i++ {
			if _, err := imp.ListUntrackedFiles(ctx, dir); err != nil {
				t.Fatal(err)
			}
		}
		out, err := osexec.Command("git", "config", "--global", "--get-all", "safe.directory").Output()
		if err != nil {
			t.Fatal(err)
		}
		abs, _ := filepath.Abs(dir)
		if n := strings.Count(string(out), abs+"\n"); n != 1 {
			t.Errorf("safe.directory has %d entries for %s: %q", n, abs, out)
		}
	})
}
