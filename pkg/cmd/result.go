package cmd

import (
	"sync"
	"time"

	"github.com/ShotaKitazawa/isucontinuous/pkg/localrepo"
	"github.com/ShotaKitazawa/isucontinuous/pkg/usecases/deploy"
)

type HostResult struct {
	Host  string
	Tasks []string
}

type Result struct {
	Hosts    []HostResult
	Warnings []string
}

type InitResult struct{ Result }

type SetupResult struct{ Result }

type ImportResult struct{ Result }

type SyncResult struct{ Result }

type ProfilingResult struct{ Result }

// Artifact is a file that afterbench produced on a host. Size is the original byte count;
// Note is set when a front end shortened or dropped Content.
type Artifact struct {
	Host    string
	Path    string
	Size    int
	Content string
	Note    string
}

type AfterBenchResult struct {
	Result
	Artifacts []Artifact
}

type DeployResult struct {
	Result
	Revision string
	Hash     string
	Message  string
	Date     time.Time
}

type DiffResult struct {
	Result
	Diffs []deploy.HostDiff
}

type PushResult struct {
	Result
	Files []localrepo.PushedFile
}

type warningList struct {
	mu   sync.Mutex
	list []string
}

func (w *warningList) Add(msg string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.list = append(w.list, msg)
}

func (w *warningList) AddUnique(msg string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	for _, existing := range w.list {
		if existing == msg {
			return
		}
	}
	w.list = append(w.list, msg)
}

func (w *warningList) List() []string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return append([]string(nil), w.list...)
}
