package localrepo

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"go.uber.org/zap/zaptest"
	kexec "k8s.io/utils/exec"
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
