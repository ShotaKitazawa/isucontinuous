package cmd

import (
	"sync"

	"github.com/ShotaKitazawa/isucontinuous/pkg/localrepo"
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

type AfterBenchResult struct{ Result }

type DeployResult struct {
	Result
	Revision string
	Hash     string
	Message  string
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

func (w *warningList) List() []string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return append([]string(nil), w.list...)
}
