package cmd

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"go.uber.org/zap/zaptest"

	"github.com/ShotaKitazawa/isucontinuous/pkg/config"
	myerrors "github.com/ShotaKitazawa/isucontinuous/pkg/errors"
)

func TestPerHostExec(t *testing.T) {
	ctx := context.Background()
	hosts := []config.Host{{Name: "host01"}}
	ok := func(ctx context.Context, host config.Host) error { return nil }
	detached := myerrors.NewErrorGitBranchIsDetached()

	t.Run("success", func(t *testing.T) {
		rep := NewRecordingReporter()
		got, err := perHostExec(zaptest.NewLogger(t), ctx, rep, hosts, []task{{"A", ok}, {"B", ok}})
		if err != nil {
			t.Fatal(err)
		}
		if want := []HostResult{{Host: "host01", Tasks: []string{"A", "B"}}}; !reflect.DeepEqual(got, want) {
			t.Errorf("results = %+v, want %+v", got, want)
		}
		want := []Event{
			{EventStarted, "host01", "A", nil}, {EventDone, "host01", "A", nil},
			{EventStarted, "host01", "B", nil}, {EventDone, "host01", "B", nil},
		}
		if !reflect.DeepEqual(rep.Events(), want) {
			t.Errorf("events = %+v, want %+v", rep.Events(), want)
		}
	})

	t.Run("failure", func(t *testing.T) {
		rep := NewRecordingReporter()
		fail := func(ctx context.Context, host config.Host) error { return detached }
		got, err := perHostExec(zaptest.NewLogger(t), ctx, rep, hosts, []task{{"A", ok}, {"B", fail}, {"C", ok}})
		var ht myerrors.HostTask
		if !errors.As(err, &ht) || ht.Host != "host01" || ht.Task != "B" {
			t.Fatalf("err = %v, want HostTask{host01, B}", err)
		}
		if !errors.As(err, &myerrors.GitBranchIsDetached{}) {
			t.Errorf("GitBranchIsDetached is not extracted: %v", err)
		}
		if want := []string{"A"}; !reflect.DeepEqual(got[0].Tasks, want) {
			t.Errorf("tasks = %v, want %v", got[0].Tasks, want)
		}
		events := rep.Events()
		if last := events[len(events)-1]; last.Kind != EventFailed || last.Task != "B" || last.Err != detached {
			t.Errorf("last event = %+v", last)
		}
	})

	t.Run("failure cancels sibling", func(t *testing.T) {
		hosts := []config.Host{{Name: "host01"}, {Name: "host02"}}
		started := make(chan struct{})
		canceled := make(chan struct{})
		taskFn := func(ctx context.Context, host config.Host) error {
			if host.Name == "host01" {
				<-started
				return errors.New("boom")
			}
			close(started)
			<-ctx.Done()
			close(canceled)
			return ctx.Err()
		}
		_, _ = perHostExec(zaptest.NewLogger(t), ctx, NewRecordingReporter(), hosts, []task{{"A", taskFn}})
		select {
		case <-canceled:
		case <-time.After(time.Second):
			t.Fatal("sibling context was not canceled")
		}
	})
}
