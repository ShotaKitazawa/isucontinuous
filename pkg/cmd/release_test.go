package cmd

import (
	"context"
	"errors"
	"strings"
	"testing"

	"go.uber.org/zap/zaptest"

	"github.com/ShotaKitazawa/isucontinuous/pkg/config"
	"github.com/ShotaKitazawa/isucontinuous/pkg/hoststate"
)

func TestRunRelease(t *testing.T) {
	run := func(t *testing.T, e *lockEnv, force, disabled bool) *ReleaseResult {
		t.Helper()
		res, err := runRelease(ConfigRelease{ConfigCommon: e.common(), Force: force, IncludeDisabled: disabled},
			context.Background(), zaptest.NewLogger(t), e.repo, lockersWith(e.shells))
		if err != nil {
			t.Fatal(err)
		}
		return res
	}
	t.Run("own locks only by default", func(t *testing.T) {
		e := newLockEnv(t)
		e.lock("web01", me, "deploy")
		e.lock("web02", bob, "deploy")
		res := run(t, e, false, false)
		if res.Releases[0].State != ReleaseReleased || res.Releases[1].State != ReleaseHeldByOther ||
			res.Releases[1].Holder.Owner != bob {
			t.Errorf("releases = %+v", res.Releases)
		}
		if _, found := e.held("web02"); !found {
			t.Error("the lock of another owner was released")
		}
	})
	t.Run("not locked", func(t *testing.T) {
		e := newLockEnv(t)
		if res := run(t, e, false, false); res.Releases[0].State != ReleaseNotLocked {
			t.Errorf("releases = %+v", res.Releases)
		}
	})
	t.Run("force", func(t *testing.T) {
		e := newLockEnv(t)
		e.lock("web02", bob, "deploy")
		run(t, e, true, false)
		if _, found := e.held("web02"); found {
			t.Error("the lock remains")
		}
	})
	t.Run("disabled hosts", func(t *testing.T) {
		e := newLockEnv(t)
		e.shells["web03"] = hoststate.NewFakeShell("web03")
		e.lock("web03", me, "deploy")
		e.repo.conf = &config.Config{Hosts: e.hosts, Disabled: []config.DisabledHost{{Name: "web03", Host: config.Host{Name: "web03"}}}}
		if res := run(t, e, false, false); len(res.Releases) != 2 {
			t.Errorf("releases = %+v", res.Releases)
		}
		if _, found := e.held("web03"); !found {
			t.Fatal("a disabled host was released without --include-disabled")
		}
		res := run(t, e, false, true)
		if len(res.Releases) != 3 || res.Releases[2].Host != "web03" || res.Releases[2].State != ReleaseReleased {
			t.Errorf("releases = %+v", res.Releases)
		}
	})
	t.Run("unresolved disabled host", func(t *testing.T) {
		e := newLockEnv(t)
		e.repo.conf = &config.Config{Hosts: e.hosts, Disabled: []config.DisabledHost{{Name: "web03", Err: errors.New("env")}}}
		_, err := runRelease(ConfigRelease{ConfigCommon: e.common(), IncludeDisabled: true},
			context.Background(), zaptest.NewLogger(t), e.repo, lockersWith(e.shells))
		if err == nil || !strings.Contains(err.Error(), "web03") {
			t.Errorf("err = %v", err)
		}
	})
}

func TestRunStatus(t *testing.T) {
	e := newLockEnv(t)
	e.lock("web01", bob, "deploy")
	if err := hoststate.WriteLastRevision(context.Background(), e.shells["web02"], "1111111"); err != nil {
		t.Fatal(err)
	}
	e.repo.conf = &config.Config{Hosts: e.hosts, Disabled: []config.DisabledHost{{Name: "web03"}}}
	res, err := runStatus(ConfigStatus{ConfigCommon: e.common()}, context.Background(), zaptest.NewLogger(t),
		e.repo, lockersWith(e.shells))
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Statuses) != 3 {
		t.Fatalf("statuses = %+v", res.Statuses)
	}
	if s := res.Statuses[0]; s.Host != "web01" || s.Lock == nil || s.Lock.Owner != bob || s.LastRevision != "" {
		t.Errorf("web01 = %+v", s)
	}
	if s := res.Statuses[1]; s.Lock != nil || s.LastRevision != "1111111" {
		t.Errorf("web02 = %+v", s)
	}
	if s := res.Statuses[2]; s.Host != "web03" || !s.Disabled {
		t.Errorf("web03 = %+v", s)
	}
}
