package localrepo

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"go.uber.org/zap/zaptest"
	kexec "k8s.io/utils/exec"

	myerrors "github.com/ShotaKitazawa/isucontinuous/pkg/errors"
)

func TestValidateRef(t *testing.T) {
	tests := []struct {
		ref     string
		wantErr bool
	}{
		{ref: "master"},
		{ref: "feature/foo-1.2"},
		{ref: "v1.0.0"},
		{ref: "0123456789abcdef0123456789abcdef01234567"},
		{ref: "", wantErr: true},
		{ref: "-d", wantErr: true},
		{ref: "--upload-pack=x", wantErr: true},
		{ref: "a b", wantErr: true},
		{ref: "a;rm -rf /", wantErr: true},
		{ref: "a\nb", wantErr: true},
		{ref: "a..b", wantErr: true},
		{ref: "a@{1}", wantErr: true},
		{ref: "HEAD~1", wantErr: true},
		{ref: "a^", wantErr: true},
		{ref: "a:b", wantErr: true},
		{ref: "a\\b", wantErr: true},
		{ref: "a*", wantErr: true},
		{ref: "/a", wantErr: true},
		{ref: "a/", wantErr: true},
		{ref: "a//b", wantErr: true},
		{ref: "a.", wantErr: true},
		{ref: "a/.b", wantErr: true},
		{ref: "a.lock", wantErr: true},
		{ref: "@", wantErr: true},
	}
	for _, tt := range tests {
		if err := validateRef(tt.ref); (err != nil) != tt.wantErr {
			t.Errorf("validateRef(%q) error = %v, wantErr %v", tt.ref, err, tt.wantErr)
		}
	}
}

func runGit(t *testing.T, dir string, args ...string) string {
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
	return string(out)
}

// newTestRepo returns a LocalRepo cloned from a bare remote that already has one commit on master.
func newTestRepo(t *testing.T) (*LocalRepo, string, string) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed")
	}
	root := t.TempDir()
	remote := filepath.Join(root, "remote.git")
	runGit(t, root, "init", "--bare", "-b", "master", remote)
	seed := filepath.Join(root, "seed")
	runGit(t, root, "clone", remote, seed)
	runGit(t, seed, "checkout", "-b", "master")
	if err := os.WriteFile(filepath.Join(seed, "a.txt"), []byte("a"), 0644); err != nil {
		t.Fatal(err)
	}
	runGit(t, seed, "add", "-A")
	runGit(t, seed, "commit", "-m", "init")
	runGit(t, seed, "push", "origin", "master")
	work := filepath.Join(root, "work")
	runGit(t, root, "clone", remote, work)
	runGit(t, work, "config", "user.name", "test")
	runGit(t, work, "config", "user.email", "test@example.com")
	repo, err := AttachLocalRepo(zaptest.NewLogger(t), kexec.New(), work)
	if err != nil {
		t.Fatal(err)
	}
	return repo, work, remote
}

func TestLocalRepo_SwitchAndMerge(t *testing.T) {
	ctx := context.Background()
	repo, work, _ := newTestRepo(t)
	runGit(t, work, "branch", "other")
	runGit(t, work, "push", "origin", "other")
	if err := repo.SwitchAndMerge(ctx, "other"); err != nil {
		t.Fatal(err)
	}
	if b, _ := repo.CurrentBranch(ctx); b != "other" {
		t.Errorf("CurrentBranch = %q, want other", b)
	}
	if err := repo.SwitchAndMerge(ctx, "master"); err != nil {
		t.Fatal(err)
	}
	if b, _ := repo.CurrentBranch(ctx); b != "master" {
		t.Errorf("CurrentBranch = %q, want master", b)
	}
	// valid as a ref name, so it must reach git as a plain argument
	if err := repo.SwitchAndMerge(ctx, "$(touch pwned)"); err == nil {
		t.Error("expected an error for an unknown branch")
	}
	if err := repo.SwitchAndMerge(ctx, "a b"); err == nil {
		t.Error("expected an error for an invalid branch name")
	}
	if _, err := os.Stat(filepath.Join(work, "pwned")); err == nil {
		t.Error("branch name was executed as shell")
	}
}

func TestLocalRepo_SwitchDetachedBranch(t *testing.T) {
	ctx := context.Background()
	repo, work, _ := newTestRepo(t)
	if err := repo.SwitchDetachedBranch(ctx, "master"); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.CurrentBranch(ctx); err == nil {
		t.Error("expected detached HEAD")
	}
	head := runGit(t, work, "rev-parse", "HEAD")
	if err := repo.SwitchDetachedBranch(ctx, head[:40]); err != nil {
		t.Fatal(err)
	}
	if err := repo.SwitchDetachedBranch(ctx, "master; touch pwned"); err == nil {
		t.Error("expected an error for an invalid revision")
	}
	if _, err := os.Stat(filepath.Join(work, "pwned")); err == nil {
		t.Error("revision was executed as shell")
	}
	if err := repo.SwitchDetachedBranch(ctx, "no-such-revision"); err == nil {
		t.Error("expected an error for an unknown revision")
	}
}

func TestLocalRepo_Push(t *testing.T) {
	ctx := context.Background()
	t.Run("no changes", func(t *testing.T) {
		repo, _, _ := newTestRepo(t)
		result, err := repo.Push(ctx, "")
		if err != nil {
			t.Fatal(err)
		}
		if len(result.Files) != 0 || len(result.Warnings) != 0 {
			t.Errorf("unexpected result: %+v", result)
		}
	})
	t.Run("commit message and files", func(t *testing.T) {
		repo, work, remote := newTestRepo(t)
		if err := os.WriteFile(filepath.Join(work, "b.txt"), []byte("hello"), 0644); err != nil {
			t.Fatal(err)
		}
		if err := os.Remove(filepath.Join(work, "a.txt")); err != nil {
			t.Fatal(err)
		}
		msg := "it's \"quoted\" $(touch pwned)\nsecond line"
		result, err := repo.Push(ctx, msg)
		if err != nil {
			t.Fatal(err)
		}
		want := []PushedFile{{Path: "a.txt", Deleted: true}, {Path: "b.txt", Size: 5}}
		if len(result.Files) != len(want) {
			t.Fatalf("Files = %+v, want %+v", result.Files, want)
		}
		for i := range want {
			if result.Files[i] != want[i] {
				t.Errorf("Files[%d] = %+v, want %+v", i, result.Files[i], want[i])
			}
		}
		if got := runGit(t, remote, "log", "-1", "--format=%B", "master"); got != msg+"\n\n" {
			t.Errorf("commit message = %q, want %q", got, msg+"\n\n")
		}
		if _, err := os.Stat(filepath.Join(work, "pwned")); err == nil {
			t.Error("message was executed as shell")
		}
	})
	t.Run("default message", func(t *testing.T) {
		repo, work, remote := newTestRepo(t)
		if err := os.WriteFile(filepath.Join(work, "b.txt"), []byte("x"), 0644); err != nil {
			t.Fatal(err)
		}
		if _, err := repo.Push(ctx, ""); err != nil {
			t.Fatal(err)
		}
		if got := runGit(t, remote, "log", "-1", "--format=%s", "master"); got != "commit by isucontinuous\n" {
			t.Errorf("commit message = %q", got)
		}
	})
	t.Run("large file warning", func(t *testing.T) {
		repo, work, _ := newTestRepo(t)
		if err := os.WriteFile(filepath.Join(work, "big.bin"), make([]byte, 1<<20), 0644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(work, "small.txt"), []byte("x"), 0644); err != nil {
			t.Fatal(err)
		}
		result, err := repo.Push(ctx, "")
		if err != nil {
			t.Fatal(err)
		}
		if len(result.Warnings) != 1 {
			t.Errorf("Warnings = %v, want 1 warning", result.Warnings)
		}
	})
	t.Run("push failure is returned", func(t *testing.T) {
		repo, work, _ := newTestRepo(t)
		runGit(t, work, "remote", "set-url", "origin", filepath.Join(work, "no-such-remote"))
		if err := os.WriteFile(filepath.Join(work, "b.txt"), []byte("x"), 0644); err != nil {
			t.Fatal(err)
		}
		if _, err := repo.Push(ctx, ""); err == nil {
			t.Error("expected an error when git push fails")
		}
	})
	t.Run("commit failure is returned", func(t *testing.T) {
		repo, work, _ := newTestRepo(t)
		hook := filepath.Join(work, ".git", "hooks", "pre-commit")
		if err := os.WriteFile(hook, []byte("#!/bin/sh\nexit 1\n"), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(work, "b.txt"), []byte("x"), 0644); err != nil {
			t.Fatal(err)
		}
		if _, err := repo.Push(ctx, ""); err == nil {
			t.Error("expected an error when git commit fails")
		}
	})
}

func TestLocalRepo_LoadConf(t *testing.T) {
	dir := t.TempDir()
	yaml := "hosts:\n- host: 192.0.2.1\n  name: isu1\n- host: '{{.Env.LOCALREPO_TEST_NOT_SET}}'\n  disabled: true\n"
	if err := os.WriteFile(filepath.Join(dir, isucontinuousFilename), []byte(yaml), 0644); err != nil {
		t.Fatal(err)
	}
	repo, err := AttachLocalRepo(zaptest.NewLogger(t), kexec.New(), dir)
	if err != nil {
		t.Fatal(err)
	}
	conf, err := repo.LoadConf()
	if err != nil {
		t.Fatal(err)
	}
	if len(conf.Hosts) != 1 || conf.Hosts[0].Name != "isu1" {
		t.Errorf("hosts = %+v", conf.Hosts)
	}
	if ok, err := repo.HasDisabledHost(); err != nil || !ok {
		t.Errorf("HasDisabledHost = %v, %v; want true", ok, err)
	}
}

func TestInitLocalRepo_SshCommand(t *testing.T) {
	for _, c := range []struct{ key, want string }{
		{"/keys/deploy key", "ssh -i '/keys/deploy key' -o IdentitiesOnly=yes"},
		{"", ""},
	} {
		dir := filepath.Join(t.TempDir(), "repo")
		if err := os.Mkdir(dir, 0755); err != nil {
			t.Fatal(err)
		}
		if _, err := InitLocalRepo(zaptest.NewLogger(t), kexec.New(), dir, "u", "u@example.com", "git@example.com:a/b.git", c.key); err != nil {
			t.Fatal(err)
		}
		out, _ := exec.Command("git", "-C", dir, "config", "--local", "--get", "core.sshCommand").Output()
		if got := strings.TrimSpace(string(out)); got != c.want {
			t.Errorf("key=%q: core.sshCommand = %q, want %q", c.key, got, c.want)
		}
	}
	dir := filepath.Join(t.TempDir(), "repo")
	os.Mkdir(dir, 0755)
	if _, err := InitLocalRepo(zaptest.NewLogger(t), kexec.New(), dir, "u", "e", "r", "/a'b"); err == nil {
		t.Error("a key path with a single quote must be rejected")
	}
}

func TestLocalRepo_ResolveRevisionAndShowFile(t *testing.T) {
	ctx := context.Background()
	repo, work, _ := newTestRepo(t)
	first := strings.TrimSpace(runGit(t, work, "rev-parse", "HEAD"))
	if err := os.WriteFile(filepath.Join(work, "a.txt"), []byte("a2\n\n"), 0644); err != nil {
		t.Fatal(err)
	}
	runGit(t, work, "commit", "-am", "second")
	runGit(t, work, "push", "origin", "master")
	second := strings.TrimSpace(runGit(t, work, "rev-parse", "HEAD"))

	for _, rev := range []string{first, "master"} {
		want := first
		if rev == "master" {
			want = second
		}
		got, err := repo.ResolveRevision(ctx, rev)
		if err != nil || got != want {
			t.Errorf("ResolveRevision(%q) = %q, %v; want %q", rev, got, err, want)
		}
	}
	if _, err := repo.ResolveRevision(ctx, "no-such-revision"); err == nil {
		t.Error("expected an error for an unknown revision")
	}
	if _, err := repo.ResolveRevision(ctx, "--output=x"); err == nil {
		t.Error("expected an error for an invalid revision")
	}

	content, found, err := repo.ShowFile(ctx, first, "a.txt")
	if err != nil || !found || string(content) != "a" {
		t.Errorf("ShowFile(first) = %q, %v, %v", content, found, err)
	}
	content, _, _ = repo.ShowFile(ctx, second, "a.txt")
	if string(content) != "a2\n\n" {
		t.Errorf("trailing newlines must be kept: %q", content)
	}
	if _, found, err := repo.ShowFile(ctx, first, "no/such/file"); found || err != nil {
		t.Errorf("ShowFile(missing) = %v, %v", found, err)
	}
	if head := strings.TrimSpace(runGit(t, work, "rev-parse", "--abbrev-ref", "HEAD")); head != "master" {
		t.Errorf("working tree moved: %s", head)
	}
}

func TestLocalRepo_IsClean(t *testing.T) {
	ctx := context.Background()
	repo, work, _ := newTestRepo(t)
	if paths, err := repo.IsClean(ctx, true); err != nil || len(paths) != 0 {
		t.Fatalf("paths = %v, err = %v", paths, err)
	}
	if err := os.WriteFile(filepath.Join(work, ".gitignore"), []byte("ignored.txt\n"), 0644); err != nil {
		t.Fatal(err)
	}
	runGit(t, work, "add", ".gitignore")
	runGit(t, work, "commit", "-m", "ignore")
	os.WriteFile(filepath.Join(work, "ignored.txt"), []byte("x"), 0644)
	if paths, _ := repo.IsClean(ctx, false); len(paths) != 0 {
		t.Errorf("ignored file must not count without includeIgnored: %v", paths)
	}
	if paths, _ := repo.IsClean(ctx, true); len(paths) != 1 || paths[0] != "ignored.txt" {
		t.Errorf("paths = %v, want [ignored.txt]", paths)
	}
	os.MkdirAll(filepath.Join(work, "dir"), 0755)
	os.WriteFile(filepath.Join(work, "dir", "new.txt"), []byte("x"), 0644)
	os.WriteFile(filepath.Join(work, "a.txt"), []byte("changed"), 0644)
	paths, err := repo.IsClean(ctx, false)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(paths, ",") != "a.txt,dir/new.txt" {
		t.Errorf("paths = %v", paths)
	}
}

func TestLocalRepo_GetHeadCommit(t *testing.T) {
	repo, work, _ := newTestRepo(t)
	runGit(t, work, "commit", "--allow-empty", "-m", "subject\n\nbody")
	c, err := repo.GetHeadCommit(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if want := strings.TrimSpace(runGit(t, work, "rev-parse", "HEAD")); c.Hash != want {
		t.Errorf("Hash = %q, want %q", c.Hash, want)
	}
	if c.Message != "subject\n\nbody" || c.Date.IsZero() {
		t.Errorf("commit = %+v", c)
	}
}

func TestLocalRepo_RemoteBranches(t *testing.T) {
	repo, work, _ := newTestRepo(t)
	runGit(t, work, "branch", "other")
	runGit(t, work, "push", "origin", "other")
	branches, def, err := repo.RemoteBranches(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(branches, ",") != "master,other" || def != "master" {
		t.Errorf("branches = %v, default = %q", branches, def)
	}
}

func TestLocalRepo_Lock(t *testing.T) {
	ctx := context.Background()
	repo, work, _ := newTestRepo(t)
	unlock, err := repo.Lock(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(work, ".git", "isucontinuous.lock")); err != nil {
		t.Errorf("lock file must be in .git: %v", err)
	}
	if paths, _ := repo.IsClean(ctx, true); len(paths) != 0 {
		t.Errorf("lock file must not dirty the working tree: %v", paths)
	}
	_, err = repo.Lock(ctx)
	var locked myerrors.LocalRepoLocked
	if !errors.As(err, &locked) {
		t.Fatalf("err = %v, want LocalRepoLocked", err)
	}
	unlock()
	unlock2, err := repo.Lock(ctx)
	if err != nil {
		t.Fatalf("lock after unlock: %v", err)
	}
	unlock2()
}
