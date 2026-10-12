package hoststate

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/golang/mock/gomock"

	mock_shell "github.com/ShotaKitazawa/isucontinuous/pkg/shell/mock"
)

var (
	alice = Lock{Owner: "alice@pc", Revision: hash, AcquiredAt: time.Date(2026, 10, 13, 1, 2, 3, 0, time.UTC), Operation: "deploy"}
	bob   = Lock{Owner: "bob@pc", Revision: hash, AcquiredAt: alice.AcquiredAt, Operation: "deploy"}
)

func TestAcquireLock_Commands(t *testing.T) {
	ctx := context.Background()
	m := mock_shell.NewMockIface(gomock.NewController(t))
	gomock.InOrder(
		m.EXPECT().Execf(ctx, "", readLockFmt, "'/var/lib/isucontinuous'", "'/tmp/isucontinuous'").
			Return(bytes.Buffer{}, bytes.Buffer{}, nil),
		m.EXPECT().Execf(ctx, "", createLockFmt, "'/var/lib/isucontinuous'", "'/var/lib/isucontinuous/lock'").
			Return(bytes.Buffer{}, bytes.Buffer{}, nil),
		m.EXPECT().Execf(ctx, "", writeInfoFmt,
			"'owner=alice@pc\nrevision="+hash+"\nacquired_at=2026-10-13T01:02:03Z\noperation=deploy'",
			"'/var/lib/isucontinuous/lock/info'").Return(bytes.Buffer{}, bytes.Buffer{}, nil),
	)
	if out, err := AcquireLock(ctx, m, alice, AcquireOptions{}); err != nil || out != Created {
		t.Fatalf("out = %v, err = %v", out, err)
	}
}

func TestAcquireLock(t *testing.T) {
	ctx := context.Background()
	t.Run("created in primary", func(t *testing.T) {
		f := NewFakeShell("h")
		if _, err := AcquireLock(ctx, f, alice, AcquireOptions{}); err != nil {
			t.Fatal(err)
		}
		got, found, err := ReadLock(ctx, f)
		if err != nil || !found || got != alice {
			t.Errorf("got %+v, %v, %v", got, found, err)
		}
	})
	t.Run("fallback when primary is not writable", func(t *testing.T) {
		f := NewFakeShell("h")
		f.PrimaryWritable = false
		if out, err := AcquireLock(ctx, f, alice, AcquireOptions{}); err != nil || out != Created {
			t.Fatal(out, err)
		}
		if _, dir, found, _ := readLock(ctx, f); !found || dir != DirFallback {
			t.Errorf("dir = %q", dir)
		}
		// the other owner sees it even though its primary is writable again
		f.PrimaryWritable = true
		var locked *LockedError
		if _, err := AcquireLock(ctx, f, bob, AcquireOptions{}); !errors.As(err, &locked) || locked.Holder.Owner != alice.Owner {
			t.Errorf("err = %v", err)
		}
	})
	t.Run("both unwritable", func(t *testing.T) {
		m := mock_shell.NewMockIface(gomock.NewController(t))
		m.EXPECT().Host().Return("h").AnyTimes()
		m.EXPECT().Execf(ctx, "", readLockFmt, gomock.Any(), gomock.Any()).Return(bytes.Buffer{}, bytes.Buffer{}, nil).Times(3)
		m.EXPECT().Execf(ctx, "", createLockFmt, gomock.Any(), gomock.Any()).
			Return(bytes.Buffer{}, *bytes.NewBufferString("denied"), errors.New("denied")).Times(2)
		if _, err := AcquireLock(ctx, m, alice, AcquireOptions{}); err == nil || !strings.Contains(err.Error(), "denied") {
			t.Errorf("err = %v", err)
		}
	})
	t.Run("held by another owner", func(t *testing.T) {
		f := NewFakeShell("web01")
		if _, err := AcquireLock(ctx, f, alice, AcquireOptions{}); err != nil {
			t.Fatal(err)
		}
		_, err := AcquireLock(ctx, f, bob, AcquireOptions{})
		var locked *LockedError
		if !errors.As(err, &locked) || locked.Host != "web01" || locked.Holder != alice {
			t.Fatalf("err = %v", err)
		}
		if !strings.Contains(err.Error(), "alice@pc") {
			t.Errorf("message does not name the holder: %v", err)
		}
	})
	t.Run("reentrant keeps the content unless refreshed", func(t *testing.T) {
		f := NewFakeShell("h")
		if _, err := AcquireLock(ctx, f, alice, AcquireOptions{}); err != nil {
			t.Fatal(err)
		}
		next := alice
		next.Operation = "setup"
		if out, err := AcquireLock(ctx, f, next, AcquireOptions{}); err != nil || out != Reentered {
			t.Fatal(out, err)
		}
		if got, _, _ := ReadLock(ctx, f); got.Operation != "deploy" {
			t.Errorf("content was rewritten: %+v", got)
		}
		if out, err := AcquireLock(ctx, f, next, AcquireOptions{Refresh: true}); err != nil || out != Reentered {
			t.Fatal(out, err)
		}
		if got, _, _ := ReadLock(ctx, f); got.Operation != "setup" {
			t.Errorf("content was not rewritten: %+v", got)
		}
	})
	t.Run("force takes over", func(t *testing.T) {
		f := NewFakeShell("h")
		if _, err := AcquireLock(ctx, f, alice, AcquireOptions{}); err != nil {
			t.Fatal(err)
		}
		if out, err := AcquireLock(ctx, f, bob, AcquireOptions{Force: true}); err != nil || out != TookOver {
			t.Fatal(out, err)
		}
		if got, _, _ := ReadLock(ctx, f); got.Owner != bob.Owner {
			t.Errorf("got %+v", got)
		}
	})
	t.Run("lock without content is held by unknown", func(t *testing.T) {
		f := NewFakeShell("h")
		f.dirs[DirPrimary+"/lock"] = true
		var locked *LockedError
		if _, err := AcquireLock(ctx, f, alice, AcquireOptions{}); !errors.As(err, &locked) || locked.Holder.Owner != "" {
			t.Errorf("err = %v", err)
		}
	})
	t.Run("invalid values", func(t *testing.T) {
		f := NewFakeShell("h")
		for _, l := range []Lock{
			{Owner: "a\nb", Revision: hash}, {Owner: "", Revision: hash}, {Owner: "a@b", Revision: "x; rm -rf /"},
		} {
			if _, err := AcquireLock(ctx, f, l, AcquireOptions{}); err == nil {
				t.Errorf("%+v: want error", l)
			}
		}
	})
}

func TestReleaseLock(t *testing.T) {
	ctx := context.Background()
	f := NewFakeShell("h")
	if out, _, err := ReleaseLock(ctx, f, alice.Owner, false); err != nil || out != NotLocked {
		t.Fatal(out, err)
	}
	if _, err := AcquireLock(ctx, f, alice, AcquireOptions{}); err != nil {
		t.Fatal(err)
	}
	if out, holder, err := ReleaseLock(ctx, f, bob.Owner, false); err != nil || out != HeldByOther || holder.Owner != alice.Owner {
		t.Fatal(out, holder, err)
	}
	if _, found, _ := ReadLock(ctx, f); !found {
		t.Fatal("the lock of another owner was released")
	}
	if out, _, err := ReleaseLock(ctx, f, bob.Owner, true); err != nil || out != Released {
		t.Fatal(out, err)
	}
	if _, found, _ := ReadLock(ctx, f); found {
		t.Fatal("the lock remains")
	}
	_, _ = AcquireLock(ctx, f, alice, AcquireOptions{})
	if out, _, err := ReleaseLock(ctx, f, alice.Owner, false); err != nil || out != Released {
		t.Fatal(out, err)
	}
}

func TestRequireLock(t *testing.T) {
	ctx := context.Background()
	f := NewFakeShell("web01")
	var notLocked *NotLockedError
	if _, err := RequireLock(ctx, f, alice.Owner); !errors.As(err, &notLocked) || notLocked.Host != "web01" {
		t.Fatalf("err = %v", err)
	}
	_, _ = AcquireLock(ctx, f, alice, AcquireOptions{})
	if got, err := RequireLock(ctx, f, alice.Owner); err != nil || got.Revision != hash {
		t.Fatal(got, err)
	}
	var locked *LockedError
	if _, err := RequireLock(ctx, f, bob.Owner); !errors.As(err, &locked) {
		t.Fatalf("err = %v", err)
	}
}
