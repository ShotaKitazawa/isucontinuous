package cmd

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"

	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"

	"github.com/ShotaKitazawa/isucontinuous/pkg/config"
	"github.com/ShotaKitazawa/isucontinuous/pkg/localrepo"
	"github.com/ShotaKitazawa/isucontinuous/pkg/usecases/imports"
	"github.com/ShotaKitazawa/isucontinuous/pkg/usecases/lock"
)

type writePathRepo struct {
	*fakeRepo
	err error
}

func (r *writePathRepo) ValidateWritePath(string) error { return r.err }

func TestCheckImportDestination(t *testing.T) {
	t.Run("symlink is skipped and warned once", func(t *testing.T) {
		repo := &writePathRepo{fakeRepo: &fakeRepo{}, err: &localrepo.SymlinkInWritePath{Path: "isu1/shared"}}
		warnings := &warningList{}
		for range 2 {
			skip, err := checkImportDestination(repo, warnings, "isu1", "isu1/shared/app")
			if err != nil || !skip {
				t.Fatalf("skip = %v, err = %v", skip, err)
			}
		}
		got := warnings.List()
		if len(got) != 1 || got[0] != `host isu1: import skipped destination under symlink "isu1/shared"` {
			t.Fatalf("warnings = %v", got)
		}
	})

	t.Run("other validation error is returned", func(t *testing.T) {
		want := errors.New("permission denied")
		repo := &writePathRepo{fakeRepo: &fakeRepo{}, err: want}
		skip, err := checkImportDestination(repo, &warningList{}, "isu1", "isu1/app")
		if skip || !errors.Is(err, want) {
			t.Fatalf("skip = %v, err = %v", skip, err)
		}
	})
}

func TestWarningListAddUniqueConcurrent(t *testing.T) {
	warnings := &warningList{}
	var wg sync.WaitGroup
	for i := range 20 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			warnings.AddUnique(fmt.Sprintf("warning-%d", i%2))
		}()
	}
	wg.Wait()
	if got := warnings.List(); len(got) != 2 {
		t.Fatalf("warnings = %v", got)
	}
}

func TestRunImportWorkflow(t *testing.T) {
	newImporters := func(*zap.Logger, []config.Host) (map[string]*imports.Importer, error) {
		return map[string]*imports.Importer{}, nil
	}
	newLockers := func(*zap.Logger, []config.Host) (map[string]*lock.Locker, error) {
		return map[string]*lock.Locker{}, nil
	}
	run := func(t *testing.T, repo *fakeRepo) (*ImportResult, error) {
		t.Helper()
		return runImport(ConfigImport{GitBranch: "master"}, context.Background(), zaptest.NewLogger(t), repo, newImporters, newLockers)
	}

	t.Run("existing repository syncs then imports and pushes", func(t *testing.T) {
		repo := &fakeRepo{
			branch: "feature",
			conf:   &config.Config{},
			pushed: &localrepo.PushResult{Files: []localrepo.PushedFile{{Path: "isu1/app", Size: 3}}},
		}
		result, err := run(t, repo)
		if err != nil {
			t.Fatal(err)
		}
		if repo.pushes != 1 || len(result.Files) != 1 || result.Files[0].Path != "isu1/app" {
			t.Fatalf("pushes = %d, result = %+v", repo.pushes, result)
		}
		if got := repo.calls; len(got) < 1 || got[0] != "merge:master" {
			t.Fatalf("calls = %v", got)
		}
	})

	t.Run("first import keeps edited initial files", func(t *testing.T) {
		repo := &fakeRepo{
			branch: "master",
			first:  true,
			dirty:  []string{"isucontinuous.yaml"},
			conf:   &config.Config{},
			pushed: &localrepo.PushResult{Files: []localrepo.PushedFile{{Path: "isucontinuous.yaml", Size: 10}}},
		}
		result, err := run(t, repo)
		if err != nil {
			t.Fatal(err)
		}
		if repo.pushes != 1 || len(result.Files) != 1 {
			t.Fatalf("pushes = %d, result = %+v", repo.pushes, result)
		}
		for _, call := range repo.calls {
			if call == "merge:master" {
				t.Fatalf("first import unexpectedly synced: %v", repo.calls)
			}
		}
	})

	t.Run("dirty established repository is rejected", func(t *testing.T) {
		repo := &fakeRepo{branch: "master", dirty: []string{"changed"}, conf: &config.Config{}}
		result, err := run(t, repo)
		if result != nil || err == nil || repo.pushes != 0 {
			t.Fatalf("result = %+v, err = %v, pushes = %d", result, err, repo.pushes)
		}
	})

	t.Run("acquisition failure does not push", func(t *testing.T) {
		want := errors.New("load after sync failed")
		repo := &fakeRepo{branch: "master", loadErr: want}
		result, err := run(t, repo)
		if result != nil || !errors.Is(err, want) || repo.pushes != 0 {
			t.Fatalf("result = %+v, err = %v, pushes = %d", result, err, repo.pushes)
		}
	})

	t.Run("push failure returns acquired result for recovery", func(t *testing.T) {
		want := errors.New("push failed")
		repo := &fakeRepo{branch: "master", first: true, conf: &config.Config{}, pushErr: want}
		result, err := run(t, repo)
		if result == nil || !errors.Is(err, want) || repo.pushes != 1 {
			t.Fatalf("result = %+v, err = %v, pushes = %d", result, err, repo.pushes)
		}
	})
}
