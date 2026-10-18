package cmd

import (
	"errors"
	"fmt"
	"sync"
	"testing"

	"github.com/ShotaKitazawa/isucontinuous/pkg/localrepo"
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
