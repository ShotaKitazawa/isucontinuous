package cmd

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"

	"github.com/ShotaKitazawa/isucontinuous/pkg/config"
	myerrors "github.com/ShotaKitazawa/isucontinuous/pkg/errors"
	"github.com/ShotaKitazawa/isucontinuous/pkg/hoststate"
	"github.com/ShotaKitazawa/isucontinuous/pkg/localrepo"
	"github.com/ShotaKitazawa/isucontinuous/pkg/shell"
	"github.com/ShotaKitazawa/isucontinuous/pkg/slack"
	"github.com/ShotaKitazawa/isucontinuous/pkg/template"
	"github.com/ShotaKitazawa/isucontinuous/pkg/usecases/afterbench"
	"github.com/ShotaKitazawa/isucontinuous/pkg/usecases/deploy"
	"github.com/ShotaKitazawa/isucontinuous/pkg/usecases/imports"
	"github.com/ShotaKitazawa/isucontinuous/pkg/usecases/install"
	lockusecase "github.com/ShotaKitazawa/isucontinuous/pkg/usecases/lock"
	"github.com/ShotaKitazawa/isucontinuous/pkg/usecases/profiling"
)

type cancelAwareShell struct{ shell.Iface }

func (s cancelAwareShell) Exec(ctx context.Context, basedir, command string) (bytes.Buffer, bytes.Buffer, error) {
	if err := ctx.Err(); err != nil {
		return bytes.Buffer{}, bytes.Buffer{}, err
	}
	return s.Iface.Exec(ctx, basedir, command)
}

func (s cancelAwareShell) Execf(ctx context.Context, basedir, command string, args ...interface{}) (bytes.Buffer, bytes.Buffer, error) {
	if err := ctx.Err(); err != nil {
		return bytes.Buffer{}, bytes.Buffer{}, err
	}
	return s.Iface.Execf(ctx, basedir, command, args...)
}

func (s cancelAwareShell) Deploy(ctx context.Context, src, dst string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return s.Iface.Deploy(ctx, src, dst)
}

func (s cancelAwareShell) ReadFile(ctx context.Context, path string) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return s.Iface.ReadFile(ctx, path)
}

const (
	me       = "me@pc"
	bob      = "bob@pc"
	headHash = "abc1234" // what fakeRepo.GetHeadInfo returns
)

type lockEnv struct {
	t    *testing.T
	dir  string
	repo *fakeRepo
	// lockRevision is the revision of locks made by lock(); it matches HEAD unless a test says otherwise
	lockRevision string
	hosts        []config.Host
	shells       map[string]*hoststate.FakeShell

	mu       sync.Mutex
	commands []string
}

func newLockEnv(t *testing.T) *lockEnv {
	t.Helper()
	e := &lockEnv{t: t, lockRevision: headHash, dir: t.TempDir(), shells: map[string]*hoststate.FakeShell{}}
	e.repo = &fakeRepo{shown: map[string][]byte{}}
	for _, name := range []string{"web01", "web02"} {
		if err := os.MkdirAll(filepath.Join(e.dir, name, "app"), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(e.dir, name, "app", "a.conf"), []byte("new"), 0644); err != nil {
			t.Fatal(err)
		}
		e.repo.shown[name+"/app/a.conf"] = []byte("old")
		sh := hoststate.NewFakeShell(name)
		sh.ExecError = e.record
		e.shells[name] = sh
		e.hosts = append(e.hosts, config.Host{
			Name:       name,
			Deploy:     config.Deploy{Targets: []config.DeployTarget{{Src: "app", Target: "/etc/app"}}},
			Profiling:  config.Profiling{Command: "profile {{.Git.Revision}}"},
			AfterBench: config.AfterBench{Target: "/p/{{.Git.Revision}}", Command: "collect"},
		})
	}
	e.repo.conf = &config.Config{Hosts: e.hosts}
	return e
}

func (e *lockEnv) record(command string) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.commands = append(e.commands, command)
	if strings.Contains(command, "boom") {
		return errors.New("boom")
	}
	return nil
}

func (e *lockEnv) ran(substr string) bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	for _, c := range e.commands {
		if strings.Contains(c, substr) {
			return true
		}
	}
	return false
}

func (e *lockEnv) common() ConfigCommon {
	return ConfigCommon{LocalRepoPath: e.dir, Owner: me, Reporter: NewRecordingReporter()}
}

func (e *lockEnv) lock(host, owner, operation string) {
	e.t.Helper()
	l := hoststate.Lock{Owner: owner, Revision: e.lockRevision, AcquiredAt: time.Now(), Operation: operation}
	if _, err := hoststate.AcquireLock(context.Background(), e.shells[host], l, hoststate.AcquireOptions{}); err != nil {
		e.t.Fatal(err)
	}
}

func (e *lockEnv) held(host string) (hoststate.Lock, bool) {
	e.t.Helper()
	l, found, err := hoststate.ReadLock(context.Background(), e.shells[host])
	if err != nil {
		e.t.Fatal(err)
	}
	return l, found
}

func (e *lockEnv) lastRevision(host string) string {
	e.t.Helper()
	h, _, err := hoststate.ReadLastRevision(context.Background(), e.shells[host])
	if err != nil {
		e.t.Fatal(err)
	}
	return h
}

func (e *lockEnv) deploy(conf ConfigDeploy) (*DeployResult, error) {
	conf.ConfigCommon = e.common()
	conf.GitRevision = "master"
	newDeployers := func(l *zap.Logger, t *template.Templator, repoPath string, _ []config.Host) (map[string]*deploy.Deployer, error) {
		m := map[string]*deploy.Deployer{}
		for name, sh := range e.shells {
			m[name] = deploy.NewWithShell(l, sh, t, repoPath)
		}
		return m, nil
	}
	newSlack := func(*config.Config) (slack.ClientIface, error) { return slack.NewFakeClient(zap.NewNop()), nil }
	return runDeploy(conf, context.Background(), zaptest.NewLogger(e.t), e.repo, newSlack, newDeployers, lockersWith(e.shells))
}

func TestRunDeploy_AcquiresAndKeepsLock(t *testing.T) {
	e := newLockEnv(t)
	if _, err := e.deploy(ConfigDeploy{}); err != nil {
		t.Fatal(err)
	}
	for _, h := range []string{"web01", "web02"} {
		l, found := e.held(h)
		if !found || l.Owner != me || l.Revision != headHash || l.Operation != "deploy" {
			t.Errorf("%s: lock = %+v, %v", h, l, found)
		}
		if got := e.lastRevision(h); got != headHash {
			t.Errorf("%s: last revision = %q", h, got)
		}
	}
}

func TestRunDeploy_AllOrNothing(t *testing.T) {
	t.Run("held by another owner", func(t *testing.T) {
		e := newLockEnv(t)
		e.lock("web02", bob, "deploy")
		_, err := e.deploy(ConfigDeploy{})
		var locked *hoststate.LockedError
		if !errors.As(err, &locked) || locked.Host != "web02" || locked.Holder.Owner != bob {
			t.Fatalf("err = %v", err)
		}
		if _, found := e.held("web01"); found {
			t.Error("web01 stays locked although web02 failed")
		}
		if e.lastRevision("web01") != "" {
			t.Error("something was deployed")
		}
	})
	t.Run("a lock held before is kept as it was", func(t *testing.T) {
		e := newLockEnv(t)
		e.lock("web01", me, "deploy")
		e.lock("web02", bob, "deploy")
		if _, err := e.deploy(ConfigDeploy{}); err == nil {
			t.Fatal("want error")
		}
		l, found := e.held("web01")
		if !found || l.Owner != me || l.Revision != headHash {
			t.Errorf("web01: lock = %+v, %v", l, found)
		}
	})
	t.Run("reentrant refreshes the revision", func(t *testing.T) {
		e := newLockEnv(t)
		e.lockRevision = "2222222"
		e.lock("web01", me, "deploy")
		if _, err := e.deploy(ConfigDeploy{}); err != nil {
			t.Fatal(err)
		}
		if l, _ := e.held("web01"); l.Revision != headHash {
			t.Errorf("lock = %+v", l)
		}
	})
	t.Run("force takes over", func(t *testing.T) {
		e := newLockEnv(t)
		e.lock("web02", bob, "deploy")
		if _, err := e.deploy(ConfigDeploy{Force: true}); err != nil {
			t.Fatal(err)
		}
		if l, _ := e.held("web02"); l.Owner != me {
			t.Errorf("lock = %+v", l)
		}
	})
}

func TestRunDeploy_Drift(t *testing.T) {
	record := func(e *lockEnv) {
		for _, sh := range e.shells {
			if err := hoststate.WriteLastRevision(context.Background(), sh, "1111111"); err != nil {
				t.Fatal(err)
			}
		}
	}
	t.Run("fails and releases the locks", func(t *testing.T) {
		e := newLockEnv(t)
		record(e)
		_, err := e.deploy(ConfigDeploy{})
		var drift *deploy.DriftError
		if !errors.As(err, &drift) || len(drift.Diffs) != 2 {
			t.Fatalf("err = %v", err)
		}
		if !strings.Contains(err.Error(), "/etc/app/a.conf") || !strings.Contains(err.Error(), "--ignore-drift") {
			t.Errorf("message = %v", err)
		}
		for _, h := range []string{"web01", "web02"} {
			if _, found := e.held(h); found {
				t.Errorf("%s: the lock remains", h)
			}
			if e.lastRevision(h) != "1111111" {
				t.Errorf("%s: deployed despite the drift", h)
			}
		}
	})
	t.Run("ignored", func(t *testing.T) {
		e := newLockEnv(t)
		record(e)
		if _, err := e.deploy(ConfigDeploy{IgnoreDrift: true}); err != nil {
			t.Fatal(err)
		}
		if e.lastRevision("web01") != headHash {
			t.Error("not deployed")
		}
	})
	t.Run("not checked without a record", func(t *testing.T) {
		e := newLockEnv(t)
		if _, err := e.deploy(ConfigDeploy{}); err != nil {
			t.Fatal(err)
		}
	})
}

func TestRunDeploy_FailedHostIsReleased(t *testing.T) {
	e := newLockEnv(t)
	e.hosts[0].Deploy.PreCommand = "boom"
	e.repo.conf = &config.Config{Hosts: e.hosts}
	if _, err := e.deploy(ConfigDeploy{}); err == nil {
		t.Fatal("want error")
	}
	if _, found := e.held("web01"); found {
		t.Error("web01: the lock of the failed host remains")
	}
	if _, found := e.held("web02"); !found {
		t.Error("web02: the lock of the succeeded host was released")
	}
}

func (e *lockEnv) profiling() (*ProfilingResult, error) {
	newProfilingers := func(l *zap.Logger, t *template.Templator, host config.Host) (*profiling.Profilinger, error) {
		return profiling.NewWithShell(l, e.shells[host.Name], t), nil
	}
	return runProfiling(ConfigProfiling{ConfigCommon: e.common()}, context.Background(), zaptest.NewLogger(e.t),
		e.repo, newProfilingers, lockersWith(e.shells))
}

func TestRunProfiling(t *testing.T) {
	t.Run("without a lock", func(t *testing.T) {
		e := newLockEnv(t)
		_, err := e.profiling()
		var notLocked *hoststate.NotLockedError
		if !errors.As(err, &notLocked) {
			t.Fatalf("err = %v", err)
		}
		if e.ran("profile") {
			t.Error("profiling ran")
		}
	})
	t.Run("locked by another owner", func(t *testing.T) {
		e := newLockEnv(t)
		e.lock("web01", me, "deploy")
		e.lock("web02", bob, "deploy")
		_, err := e.profiling()
		var locked *hoststate.LockedError
		if !errors.As(err, &locked) || e.ran("profile") {
			t.Fatalf("err = %v", err)
		}
	})
	t.Run("HEAD differs from the revision of the lock", func(t *testing.T) {
		e := newLockEnv(t)
		e.lock("web01", me, "deploy")
		e.lockRevision = "2222222"
		e.lock("web02", me, "deploy")
		_, err := e.profiling()
		var mismatch myerrors.RevisionMismatch
		if !errors.As(err, &mismatch) || mismatch.Host != "web02" || mismatch.Locked != "2222222" || mismatch.Head != headHash {
			t.Fatalf("err = %v", err)
		}
		if e.ran("profile") {
			t.Error("profiling ran")
		}
	})
	t.Run("revision of the lock", func(t *testing.T) {
		e := newLockEnv(t)
		e.lock("web01", me, "deploy")
		e.lock("web02", me, "deploy")
		if _, err := e.profiling(); err != nil {
			t.Fatal(err)
		}
		if !e.ran("profile " + headHash) {
			t.Errorf("commands = %v", e.commands)
		}
		if _, found := e.held("web01"); !found {
			t.Error("profiling released the lock")
		}
	})
}

func (e *lockEnv) afterbench() (*AfterBenchResult, error) {
	newAfterBenchers := func(l *zap.Logger, t *template.Templator, s slack.ClientIface, host config.Host) (*afterbench.AfterBencher, error) {
		return afterbench.NewWithShell(l, e.shells[host.Name], t, s), nil
	}
	return runAfterBench(ConfigAfterBench{ConfigCommon: e.common()}, context.Background(), zaptest.NewLogger(e.t),
		e.repo, func(*config.Config) (slack.ClientIface, error) { return slack.NewFakeClient(zap.NewNop()), nil }, newAfterBenchers, lockersWith(e.shells))
}

func TestRunAfterBench(t *testing.T) {
	t.Run("without a lock", func(t *testing.T) {
		e := newLockEnv(t)
		_, err := e.afterbench()
		var notLocked *hoststate.NotLockedError
		if !errors.As(err, &notLocked) || e.ran("collect") {
			t.Fatalf("err = %v", err)
		}
	})
	t.Run("HEAD differs from the revision of the lock", func(t *testing.T) {
		e := newLockEnv(t)
		e.lockRevision = "2222222"
		e.lock("web01", me, "deploy")
		e.lock("web02", me, "deploy")
		_, err := e.afterbench()
		var mismatch myerrors.RevisionMismatch
		if !errors.As(err, &mismatch) || e.ran("collect") {
			t.Fatalf("err = %v", err)
		}
		if _, found := e.held("web01"); !found {
			t.Error("the lock must remain")
		}
	})
	t.Run("success releases", func(t *testing.T) {
		e := newLockEnv(t)
		e.lock("web01", me, "deploy")
		e.lock("web02", me, "deploy")
		if _, err := e.afterbench(); err != nil {
			t.Fatal(err)
		}
		if !e.ran(`mkdir -p "/p/abc1234"`) {
			t.Errorf("commands = %v", e.commands)
		}
		for _, h := range []string{"web01", "web02"} {
			if _, found := e.held(h); found {
				t.Errorf("%s: the lock remains", h)
			}
		}
	})
	t.Run("failure keeps the lock of the failed host only", func(t *testing.T) {
		e := newLockEnv(t)
		e.lock("web01", me, "deploy")
		e.lock("web02", me, "deploy")
		e.hosts[1].AfterBench.Command = "collect boom"
		e.repo.conf = &config.Config{Hosts: e.hosts}
		if _, err := e.afterbench(); err == nil {
			t.Fatal("want error")
		}
		if _, found := e.held("web01"); found {
			t.Error("web01: the lock of the succeeded host remains")
		}
		if _, found := e.held("web02"); !found {
			t.Error("web02: the lock of the failed host was released")
		}
	})
	t.Run("a host without target is released too", func(t *testing.T) {
		e := newLockEnv(t)
		e.lock("web01", me, "deploy")
		e.lock("web02", me, "deploy")
		e.hosts[0].AfterBench.Target = ""
		e.repo.conf = &config.Config{Hosts: e.hosts}
		if _, err := e.afterbench(); err != nil {
			t.Fatal(err)
		}
		if _, found := e.held("web01"); found {
			t.Error("web01: the lock remains")
		}
	})
}

func (e *lockEnv) bareHosts() {
	for i := range e.hosts {
		e.hosts[i].Deploy.Targets = nil
	}
	e.repo.conf = &config.Config{Hosts: e.hosts}
}

func (e *lockEnv) setup() (*SetupResult, error) {
	newInstallers := func(*zap.Logger, []config.Host) (map[string]*install.Installer, error) {
		e.record("install")
		return map[string]*install.Installer{}, nil
	}
	return runSetup(ConfigSetup{ConfigCommon: e.common()}, context.Background(), zaptest.NewLogger(e.t),
		e.repo, newInstallers, lockersWith(e.shells))
}

func (e *lockEnv) importFiles() (*ImportResult, error) {
	newImporters := func(*zap.Logger, []config.Host) (map[string]*imports.Importer, error) {
		e.record("import")
		return map[string]*imports.Importer{}, nil
	}
	return runImport(ConfigImport{ConfigCommon: e.common()}, context.Background(), zaptest.NewLogger(e.t),
		e.repo, newImporters, lockersWith(e.shells))
}

func TestRunSetupAndImport_Lock(t *testing.T) {
	cases := map[string]struct {
		run  func(*lockEnv) error
		work string
	}{
		"setup":  {func(e *lockEnv) error { _, err := e.setup(); return err }, "install"},
		"import": {func(e *lockEnv) error { _, err := e.importFiles(); return err }, "import"},
	}
	for name, c := range cases {
		c := c
		t.Run(name+" locks during the command and releases", func(t *testing.T) {
			e := newLockEnv(t)
			e.bareHosts()
			if err := c.run(e); err != nil {
				t.Fatal(err)
			}
			for _, h := range []string{"web01", "web02"} {
				if _, found := e.held(h); found {
					t.Errorf("%s: the lock remains", h)
				}
			}
			if !e.ran(c.work) {
				t.Error("the command did not run")
			}
		})
		t.Run(name+" is refused while another owner holds a host", func(t *testing.T) {
			e := newLockEnv(t)
			e.bareHosts()
			e.lock("web02", bob, "deploy")
			err := c.run(e)
			var locked *hoststate.LockedError
			if !errors.As(err, &locked) || e.ran(c.work) {
				t.Fatalf("err = %v", err)
			}
			if _, found := e.held("web01"); found {
				t.Error("web01 stays locked")
			}
			if l, _ := e.held("web02"); l.Owner != bob {
				t.Errorf("web02: lock = %+v", l)
			}
		})
		t.Run(name+" keeps the lock of a deploy", func(t *testing.T) {
			e := newLockEnv(t)
			e.bareHosts()
			e.lock("web01", me, "deploy")
			e.lock("web02", me, "deploy")
			if err := c.run(e); err != nil {
				t.Fatal(err)
			}
			if l, found := e.held("web01"); !found || l.Operation != "deploy" || l.Revision != headHash {
				t.Errorf("lock = %+v, %v", l, found)
			}
		})
	}
}

func TestLockForCommandReleasesAfterCancellation(t *testing.T) {
	e := newLockEnv(t)
	ctx, cancel := context.WithCancel(context.Background())
	newLockers := func(logger *zap.Logger, hosts []config.Host) (map[string]*lockusecase.Locker, error) {
		result := make(map[string]*lockusecase.Locker, len(hosts))
		for _, host := range hosts {
			result[host.Name] = lockusecase.NewWithShell(logger, cancelAwareShell{Iface: e.shells[host.Name]})
		}
		return result, nil
	}
	unlock, err := lockForCommand(ctx, zaptest.NewLogger(t), e.common(), e.repo, newLockers, e.hosts, "setup")
	if err != nil {
		t.Fatal(err)
	}
	cancel()
	unlock()
	for _, host := range e.hosts {
		if _, found := e.held(host.Name); found {
			t.Errorf("%s: lock remains after cancellation", host.Name)
		}
	}
}

var _ localrepo.LocalRepoIface = (*fakeRepo)(nil)
