package cmd

import (
	"fmt"
	"sync"

	"github.com/cheggaaa/pb"
)

type Reporter interface {
	Open(hosts []string, tasks int) error
	Close()
	TaskStarted(host, task string)
	TaskDone(host, task string)
	TaskFailed(host, task string, err error)
}

type ProgressReporter struct {
	mu    sync.Mutex
	tasks int
	bars  map[string]*pb.ProgressBar
	done  map[string]int
	pool  *pb.Pool
}

func NewProgressReporter() *ProgressReporter {
	return &ProgressReporter{}
}

func (r *ProgressReporter) Open(hosts []string, tasks int) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.tasks = tasks
	r.bars = make(map[string]*pb.ProgressBar, len(hosts))
	r.done = make(map[string]int, len(hosts))
	bars := make([]*pb.ProgressBar, len(hosts))
	for idx, host := range hosts {
		bars[idx] = pb.New(tasks).SetMaxWidth(80)
		r.bars[host] = bars[idx]
	}
	pool, err := pb.StartPool(bars...)
	if err != nil {
		return err
	}
	r.pool = pool
	return nil
}

func (r *ProgressReporter) Close() {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.pool != nil {
		_ = r.pool.Stop()
		r.pool = nil
	}
}

func (r *ProgressReporter) TaskStarted(host, task string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.bars[host].Prefix(fmt.Sprintf("[%s] %s", host, task))
}

func (r *ProgressReporter) TaskDone(host, task string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	bar := r.bars[host]
	bar.Increment()
	r.done[host]++
	if r.done[host] == r.tasks {
		bar.Prefix(fmt.Sprintf("[%s] %s", host, "Done!"))
	}
}

func (r *ProgressReporter) TaskFailed(host, task string, err error) {}

type EventKind string

const (
	EventStarted EventKind = "started"
	EventDone    EventKind = "done"
	EventFailed  EventKind = "failed"
)

type Event struct {
	Kind EventKind
	Host string
	Task string
	Err  error
}

type RecordingReporter struct {
	mu     sync.Mutex
	events []Event
}

func NewRecordingReporter() *RecordingReporter {
	return &RecordingReporter{}
}

func (r *RecordingReporter) Open(hosts []string, tasks int) error { return nil }

func (r *RecordingReporter) Close() {}

func (r *RecordingReporter) TaskStarted(host, task string) {
	r.add(Event{EventStarted, host, task, nil})
}

func (r *RecordingReporter) TaskDone(host, task string) { r.add(Event{EventDone, host, task, nil}) }

func (r *RecordingReporter) TaskFailed(host, task string, err error) {
	r.add(Event{EventFailed, host, task, err})
}

func (r *RecordingReporter) add(e Event) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = append(r.events, e)
}

func (r *RecordingReporter) Events() []Event {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]Event(nil), r.events...)
}
