package deploy

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/golang/mock/gomock"
	"go.uber.org/zap/zaptest"
	kexec "k8s.io/utils/exec"

	"github.com/ShotaKitazawa/isucontinuous/pkg/config"
	"github.com/ShotaKitazawa/isucontinuous/pkg/localrepo"
	mock_shell "github.com/ShotaKitazawa/isucontinuous/pkg/shell/mock"
)

func git(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@example.com",
		"GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@example.com")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v: %s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

// newDiffRepo commits host01/etc/{a,b,c}.conf, then adds host01/etc/new.conf in a later commit.
func newDiffRepo(t *testing.T) (repoPath string, committed string) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed")
	}
	repoPath = t.TempDir()
	git(t, repoPath, "init", "-b", "master")
	dir := filepath.Join(repoPath, "host01", "etc")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	for _, n := range []string{"a", "b", "c"} {
		if err := os.WriteFile(filepath.Join(dir, n+".conf"), []byte(n+" in repo\n"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	git(t, repoPath, "add", "-A")
	git(t, repoPath, "commit", "-m", "first")
	committed = git(t, repoPath, "rev-parse", "HEAD")
	if err := os.WriteFile(filepath.Join(dir, "new.conf"), []byte("new\n"), 0644); err != nil {
		t.Fatal(err)
	}
	// changed after the compared commit; Diff must read the commit, not the working tree
	if err := os.WriteFile(filepath.Join(dir, "a.conf"), []byte("a edited later\n"), 0644); err != nil {
		t.Fatal(err)
	}
	git(t, repoPath, "add", "-A")
	git(t, repoPath, "commit", "-m", "second")
	return repoPath, committed
}

func TestDeployer_Diff(t *testing.T) {
	ctx := context.Background()
	repoPath, committed := newDiffRepo(t)
	repo, err := localrepo.AttachLocalRepo(zaptest.NewLogger(t), kexec.New(), repoPath)
	if err != nil {
		t.Fatal(err)
	}
	targets := []config.DeployTarget{{Src: "etc", Target: "/etc/app"}}
	ok := func() (bytes.Buffer, bytes.Buffer, error) { return bytes.Buffer{}, bytes.Buffer{}, nil }
	sumOf := func(content, path string) string { return fmt.Sprintf("%s  %s\n", sha256Hex([]byte(content)), path) }
	const sumCmd = "sha256sum -- %s 2>/dev/null || true"
	// new.conf is not in the committed revision, so it is never queried
	const sumArgs = `'/etc/app/a.conf' '/etc/app/b.conf' '/etc/app/c.conf'`

	newDeployer := func(m *mock_shell.MockIface) Deployer {
		return Deployer{log: zaptest.NewLogger(t), shell: m, localRepoPath: repoPath}
	}

	t.Run("modified and missing", func(t *testing.T) {
		m := mock_shell.NewMockIface(gomock.NewController(t))
		m.EXPECT().Host().Return("10.0.0.1").AnyTimes()
		m.EXPECT().Exec(ctx, "", "command -v sha256sum >/dev/null 2>&1").Return(ok())
		m.EXPECT().Execf(ctx, "", sumCmd, sumArgs).Return(*bytes.NewBufferString(
			sumOf("a in repo\n", "/etc/app/a.conf") + sumOf("b edited on server\n", "/etc/app/b.conf")),
			bytes.Buffer{}, nil)
		m.EXPECT().ReadFile(ctx, "/etc/app/b.conf").Return([]byte("b edited on server\n"), nil)
		m.EXPECT().ReadFile(ctx, "/etc/app/c.conf").Return(nil, &os.PathError{Op: "open", Err: os.ErrNotExist})
		got, err := newDeployer(m).Diff(ctx, "host01", targets, committed, repo, false)
		if err != nil {
			t.Fatal(err)
		}
		want := &HostDiff{
			Host: "host01", Revision: committed,
			Changes: []FileChange{
				{Path: "/etc/app/b.conf", Kind: ChangeModified},
				{Path: "/etc/app/c.conf", Kind: ChangeMissing},
			},
			Skipped: []string{"host01/etc/new.conf"},
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("Diff() = %+v, want %+v", got, want)
		}
	})
	t.Run("show diff", func(t *testing.T) {
		m := mock_shell.NewMockIface(gomock.NewController(t))
		m.EXPECT().Host().Return("10.0.0.1").AnyTimes()
		m.EXPECT().Exec(ctx, "", "command -v sha256sum >/dev/null 2>&1").Return(ok())
		m.EXPECT().Execf(ctx, "", sumCmd, sumArgs).Return(*bytes.NewBufferString(
			sumOf("a in repo\n", "/etc/app/a.conf") + sumOf("b edited on server\n", "/etc/app/b.conf") +
				sumOf("c in repo\n", "/etc/app/c.conf")), bytes.Buffer{}, nil)
		m.EXPECT().ReadFile(ctx, "/etc/app/b.conf").Return([]byte("b edited on server\n"), nil)
		got, err := newDeployer(m).Diff(ctx, "host01", targets, committed, repo, true)
		if err != nil {
			t.Fatal(err)
		}
		if len(got.Changes) != 1 {
			t.Fatalf("Changes = %+v", got.Changes)
		}
		for _, want := range []string{"-b edited on server\n", "+b in repo\n", "10.0.0.1:/etc/app/b.conf"} {
			if !strings.Contains(got.Changes[0].Diff, want) {
				t.Errorf("Diff %q lacks %q", got.Changes[0].Diff, want)
			}
		}
	})
	t.Run("without sha256sum", func(t *testing.T) {
		m := mock_shell.NewMockIface(gomock.NewController(t))
		m.EXPECT().Host().Return("10.0.0.1").AnyTimes()
		m.EXPECT().Exec(ctx, "", "command -v sha256sum >/dev/null 2>&1").
			Return(bytes.Buffer{}, bytes.Buffer{}, errors.New("exit 1"))
		m.EXPECT().ReadFile(ctx, "/etc/app/a.conf").Return([]byte("a in repo\n"), nil)
		m.EXPECT().ReadFile(ctx, "/etc/app/b.conf").Return([]byte("b in repo\n"), nil)
		m.EXPECT().ReadFile(ctx, "/etc/app/c.conf").Return([]byte("changed"), nil)
		got, err := newDeployer(m).Diff(ctx, "host01", targets, committed, repo, false)
		if err != nil {
			t.Fatal(err)
		}
		if want := []FileChange{{Path: "/etc/app/c.conf", Kind: ChangeModified}}; !reflect.DeepEqual(got.Changes, want) {
			t.Errorf("Changes = %+v, want %+v", got.Changes, want)
		}
	})
}

func TestUnifiedDiff(t *testing.T) {
	got := unifiedDiff("old", "new", []byte("1\n2\n3\n4\n5\n6\n7\n8\n9\n10\n"), []byte("1\n2\n3\n4\nfive\n6\n7\n8\n9\n10\n"))
	want := "--- old\n+++ new\n@@ -2,7 +2,7 @@\n 2\n 3\n 4\n-5\n+five\n 6\n 7\n 8\n"
	if got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
	if got := unifiedDiff("a", "b", []byte("\x00"), []byte("x")); !strings.HasPrefix(got, "Binary files") {
		t.Errorf("got %q", got)
	}
}
