package shell

import (
	"context"
	"errors"
	"reflect"
	"sync"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"
)

type fakeSSHSession struct {
	mu      sync.Mutex
	started string
	startCh chan struct{}
	wait    chan error
	signals []ssh.Signal
	closed  bool
}

func (s *fakeSSHSession) Start(command string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.started = command
	if s.startCh != nil {
		close(s.startCh)
	}
	return nil
}

func (s *fakeSSHSession) Wait() error { return <-s.wait }

func (s *fakeSSHSession) Signal(signal ssh.Signal) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.signals = append(s.signals, signal)
	return nil
}

func (s *fakeSSHSession) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closed = true
	return nil
}

func TestRunSSHSessionReturnsWaitResult(t *testing.T) {
	want := errors.New("remote command failed")
	s := &fakeSSHSession{wait: make(chan error, 1)}
	s.wait <- want
	if got := runSSHSession(context.Background(), s, "echo ok"); !errors.Is(got, want) {
		t.Fatalf("error = %v, want %v", got, want)
	}
	if s.started != "echo ok" {
		t.Fatalf("command = %q", s.started)
	}
}

func TestRunSSHSessionEscalatesCancellation(t *testing.T) {
	s := &fakeSSHSession{wait: make(chan error)}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := runSSHSessionWithGracePeriod(ctx, s, "sleep 60", 10*time.Millisecond); !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context.Canceled", err)
	}
	// A context canceled before Start must not start a remote command.
	if s.started != "" || len(s.signals) != 0 || s.closed {
		t.Fatalf("pre-canceled session was touched: %+v", s)
	}

	s = &fakeSSHSession{wait: make(chan error), startCh: make(chan struct{})}
	ctx, cancel = context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- runSSHSessionWithGracePeriod(ctx, s, "sleep 60", 10*time.Millisecond) }()
	<-s.startCh
	cancel()
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context.Canceled", err)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if !reflect.DeepEqual(s.signals, []ssh.Signal{ssh.SIGINT, ssh.SIGTERM}) || !s.closed {
		t.Fatalf("signals = %v, closed = %v", s.signals, s.closed)
	}
}
