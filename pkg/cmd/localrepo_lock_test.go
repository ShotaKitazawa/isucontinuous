package cmd

import (
	"context"
	"errors"
	"os/exec"
	"path/filepath"
	"testing"

	"go.uber.org/zap/zaptest"
	kexec "k8s.io/utils/exec"

	myerrors "github.com/ShotaKitazawa/isucontinuous/pkg/errors"
	"github.com/ShotaKitazawa/isucontinuous/pkg/localrepo"
)

func TestRunSync_LocalRepoIsLocked(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed")
	}
	dir := t.TempDir()
	if out, err := exec.Command("git", "init", dir).CombinedOutput(); err != nil {
		t.Fatalf("git init: %v: %s", err, out)
	}
	repo, err := localrepo.AttachLocalRepo(zaptest.NewLogger(t), kexec.New(), dir)
	if err != nil {
		t.Fatal(err)
	}
	unlock, err := repo.Lock(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer unlock()
	conf := ConfigSync{
		ConfigCommon: ConfigCommon{LocalRepoPath: dir, LogFilename: filepath.Join(t.TempDir(), "log")},
		GitBranch:    "master",
	}
	_, err = RunSync(conf)
	var locked myerrors.LocalRepoLocked
	if !errors.As(err, &locked) {
		t.Fatalf("err = %v, want LocalRepoLocked", err)
	}
}
