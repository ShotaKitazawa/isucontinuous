package cmd

import (
	"bytes"
	"context"
	"errors"
	"io/ioutil"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"

	"github.com/ShotaKitazawa/isucontinuous/pkg/config"
	myerrors "github.com/ShotaKitazawa/isucontinuous/pkg/errors"
	"github.com/ShotaKitazawa/isucontinuous/pkg/hoststate"
	"github.com/ShotaKitazawa/isucontinuous/pkg/localrepo"
	"github.com/ShotaKitazawa/isucontinuous/pkg/shell"
	mock_shell "github.com/ShotaKitazawa/isucontinuous/pkg/shell/mock"
	"github.com/ShotaKitazawa/isucontinuous/pkg/slack"
	"github.com/ShotaKitazawa/isucontinuous/pkg/template"
	"github.com/ShotaKitazawa/isucontinuous/pkg/usecases/deploy"
	"github.com/ShotaKitazawa/isucontinuous/pkg/usecases/lock"
)

type fakeRepo struct {
	localrepo.LocalRepoIface
	branch   string
	pushed   *localrepo.PushResult
	conf     *config.Config
	disabled bool
	plain    bool
	calls    []string
	shown    map[string][]byte
	dirty    []string
	reset    bool
	detached bool
}

func (f *fakeRepo) CurrentBranch(ctx context.Context) (string, error) {
	if f.detached {
		return "", myerrors.NewErrorGitBranchIsDetached()
	}
	return f.branch, nil
}
func (f *fakeRepo) Fetch(ctx context.Context) error                  { return nil }
func (f *fakeRepo) DiffWithRemote(ctx context.Context) (bool, error) { return true, nil }
func (f *fakeRepo) Push(ctx context.Context, m string) (*localrepo.PushResult, error) {
	return f.pushed, nil
}
func (f *fakeRepo) LoadConf() (*config.Config, error)   { return f.conf, nil }
func (f *fakeRepo) HasDisabledHost() (bool, error)      { return f.disabled, nil }
func (f *fakeRepo) HasPlaintextPassword() (bool, error) { return f.plain, nil }
func (f *fakeRepo) ValidateWritePath(string) error      { return nil }
func (f *fakeRepo) CreateFile(string, []byte, os.FileMode) error {
	return nil
}
func (f *fakeRepo) SwitchDetachedBranch(ctx context.Context, r string) error {
	f.calls = append(f.calls, "switch")
	return nil
}
func (f *fakeRepo) ResolveRevision(ctx context.Context, r string) (string, error) {
	f.calls = append(f.calls, "resolve:"+r)
	return "0123456789abcdef0123456789abcdef01234567", nil
}
func (f *fakeRepo) ShowFile(ctx context.Context, hash, path string) ([]byte, bool, error) {
	content, ok := f.shown[path]
	return content, ok, nil
}
func (f *fakeRepo) IsClean(ctx context.Context, includeIgnored bool) ([]string, error) {
	return f.dirty, nil
}
func (f *fakeRepo) Reset(ctx context.Context) error {
	f.reset = true
	return nil
}
func (f *fakeRepo) SwitchAndMerge(ctx context.Context, branch string) error {
	f.calls = append(f.calls, "merge:"+branch)
	return nil
}
func (f *fakeRepo) GetHeadCommit(ctx context.Context) (*localrepo.CommitInfo, error) {
	return &localrepo.CommitInfo{Hash: "abc1234", Message: "message", Date: time.Date(2026, 10, 13, 1, 2, 3, 0, time.UTC)}, nil
}
func (f *fakeRepo) GetHeadInfo(ctx context.Context) (string, string, error) {
	return "abc1234", "message", nil
}

func assertNoStdout(t *testing.T, f func()) {
	t.Helper()
	orig := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w
	f()
	os.Stdout = orig
	_ = w.Close()
	out, _ := ioutil.ReadAll(r)
	if len(out) != 0 {
		t.Errorf("unexpected stdout: %q", out)
	}
}

func TestRunPush_Result(t *testing.T) {
	repo := &fakeRepo{
		branch: "master",
		pushed: &localrepo.PushResult{
			Files:    []localrepo.PushedFile{{Path: "a", Size: 2 << 20}, {Path: "b", Deleted: true}},
			Warnings: []string{"a is large"},
		},
	}
	var res *PushResult
	var err error
	assertNoStdout(t, func() {
		res, err = runPush(ConfigPush{GitBranch: "master"}, context.Background(), zaptest.NewLogger(t), repo)
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Files) != 2 || len(res.Warnings) != 1 || res.Warnings[0] != "a is large" {
		t.Errorf("result = %+v", res)
	}
}

func TestRunPush_DisabledWarning(t *testing.T) {
	for _, c := range []struct {
		branch   string
		disabled bool
		want     int
	}{
		{"main", true, 1}, {"master", true, 1}, {"main", false, 0}, {"feature", true, 0},
	} {
		repo := &fakeRepo{branch: c.branch, disabled: c.disabled, pushed: &localrepo.PushResult{}}
		res, err := runPush(ConfigPush{GitBranch: c.branch}, context.Background(), zaptest.NewLogger(t), repo)
		if err != nil {
			t.Fatal(err)
		}
		if len(res.Warnings) != c.want {
			t.Errorf("branch=%s disabled=%v: warnings = %v, want %d", c.branch, c.disabled, res.Warnings, c.want)
		}
	}
}

func TestRunPush_WrongBranch(t *testing.T) {
	repo := &fakeRepo{branch: ""}
	res, err := runPush(ConfigPush{GitBranch: "master"}, context.Background(), zaptest.NewLogger(t), repo)
	if res != nil || err == nil || !strings.Contains(err.Error(), "<detached>") {
		t.Errorf("res = %+v, err = %v", res, err)
	}
}

func TestRunDeploy_Result(t *testing.T) {
	repo := &fakeRepo{conf: &config.Config{Slack: config.Slack{DefaultChannelId: "C123"}}}
	conf := ConfigDeploy{GitRevision: "master"}
	conf.Reporter = NewRecordingReporter()
	newDeployers := func(*zap.Logger, *template.Templator, string, []config.Host) (map[string]*deploy.Deployer, error) {
		return map[string]*deploy.Deployer{}, nil
	}
	newSlackClient := func(*config.Config) (slack.ClientIface, error) {
		repo.calls = append(repo.calls, "slack")
		return slack.NewFakeClient(zap.NewNop()), nil
	}
	var res *DeployResult
	var err error
	assertNoStdout(t, func() {
		res, err = runDeploy(conf, context.Background(), zaptest.NewLogger(t), repo, newSlackClient, newDeployers, lockersWith(nil))
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Hash != "abc1234" || res.Message != "message" || res.Revision != "master" || res.Date.IsZero() {
		t.Errorf("result = %+v", res)
	}
	if want := "switch,slack"; strings.Join(repo.calls, ",") != want {
		t.Errorf("calls = %v, want %s", repo.calls, want)
	}
}

func lockersWith(shells map[string]*hoststate.FakeShell) lock.NewLockersFunc {
	return func(l *zap.Logger, _ []config.Host) (map[string]*lock.Locker, error) {
		lockers := make(map[string]*lock.Locker)
		for name, s := range shells {
			lockers[name] = lock.NewWithShell(l, s)
		}
		return lockers, nil
	}
}

func deployersWith(m shell.Iface, names ...string) deploy.NewDeployersFunc {
	return func(l *zap.Logger, t *template.Templator, repoPath string, _ []config.Host) (map[string]*deploy.Deployer, error) {
		deployers := make(map[string]*deploy.Deployer)
		for _, n := range names {
			deployers[n] = deploy.NewWithShell(l, m, t, repoPath)
		}
		return deployers, nil
	}
}

func TestRunDeploy_RecordsRevision(t *testing.T) {
	ctx := context.Background()
	m := mock_shell.NewMockIface(gomock.NewController(t))
	m.EXPECT().Host().Return("10.0.0.1").AnyTimes()
	m.EXPECT().Execf(gomock.Any(), "", `mkdir -p %s && printf '%%s\n' %s > %s`,
		"'/var/lib/isucontinuous'", "abc1234", "'/var/lib/isucontinuous/last-revision'").
		Return(bytes.Buffer{}, bytes.Buffer{}, nil)
	m.EXPECT().Execf(gomock.Any(), "", `rm -f %s`, "'/tmp/isucontinuous/last-revision'").
		Return(bytes.Buffer{}, bytes.Buffer{}, nil)
	repo := &fakeRepo{conf: &config.Config{Hosts: []config.Host{{Name: "web01", Host: "10.0.0.1"}}}}
	conf := ConfigDeploy{GitRevision: "master", IgnoreDrift: true}
	conf.Reporter = NewRecordingReporter()
	newSlackClient := func(*config.Config) (slack.ClientIface, error) { return slack.NewFakeClient(zap.NewNop()), nil }
	assertNoStdout(t, func() {
		if _, err := runDeploy(conf, ctx, zaptest.NewLogger(t), repo, newSlackClient, deployersWith(m, "web01"),
			lockersWith(map[string]*hoststate.FakeShell{"web01": hoststate.NewFakeShell("10.0.0.1")})); err != nil {
			t.Fatal(err)
		}
	})
}

func TestRunDeploy_SlackFailureWarnsAndContinues(t *testing.T) {
	for name, newSlackClient := range map[string]newSlackClientFunc{
		"send fails": func(*config.Config) (slack.ClientIface, error) {
			return slack.NewFailingClient(errors.New("channel_not_found")), nil
		},
		"client fails": func(*config.Config) (slack.ClientIface, error) { return nil, errors.New("slack token is not set") },
	} {
		t.Run(name, func(t *testing.T) {
			m := mock_shell.NewMockIface(gomock.NewController(t))
			m.EXPECT().Host().Return("10.0.0.1").AnyTimes()
			m.EXPECT().Execf(gomock.Any(), "", `mkdir -p %s && printf '%%s\n' %s > %s`,
				"'/var/lib/isucontinuous'", "abc1234", "'/var/lib/isucontinuous/last-revision'").
				Return(bytes.Buffer{}, bytes.Buffer{}, nil)
			m.EXPECT().Execf(gomock.Any(), "", `rm -f %s`, "'/tmp/isucontinuous/last-revision'").
				Return(bytes.Buffer{}, bytes.Buffer{}, nil)
			repo := &fakeRepo{conf: &config.Config{
				Slack: config.Slack{DefaultChannelId: "C123"},
				Hosts: []config.Host{{Name: "web01", Host: "10.0.0.1"}},
			}}
			conf := ConfigDeploy{GitRevision: "master", IgnoreDrift: true}
			conf.Reporter = NewRecordingReporter()
			res, err := runDeploy(conf, context.Background(), zaptest.NewLogger(t), repo, newSlackClient, deployersWith(m, "web01"),
				lockersWith(map[string]*hoststate.FakeShell{"web01": hoststate.NewFakeShell("10.0.0.1")}))
			if err != nil {
				t.Fatalf("deploy must continue: %v", err)
			}
			if len(res.Warnings) == 0 || !strings.HasPrefix(res.Warnings[0], "slack: ") {
				t.Errorf("warnings = %v, want slack warnings", res.Warnings)
			}
		})
	}
}

func TestRunDiff(t *testing.T) {
	ctx := context.Background()
	const lastCmd = `cat %s 2>/dev/null || cat %s 2>/dev/null || true`
	hosts := []config.Host{{Name: "web01", Host: "10.0.0.1"}}
	run := func(t *testing.T, conf ConfigDiff, m shell.Iface, repo *fakeRepo) *DiffResult {
		t.Helper()
		conf.Reporter = NewRecordingReporter()
		var res *DiffResult
		var err error
		assertNoStdout(t, func() {
			res, err = runDiff(conf, ctx, zaptest.NewLogger(t), repo, deployersWith(m, "web01"))
		})
		if err != nil {
			t.Fatal(err)
		}
		return res
	}

	t.Run("no record of the last deployed revision", func(t *testing.T) {
		m := mock_shell.NewMockIface(gomock.NewController(t))
		m.EXPECT().Execf(gomock.Any(), "", lastCmd, gomock.Any(), gomock.Any()).Return(bytes.Buffer{}, bytes.Buffer{}, nil)
		repo := &fakeRepo{conf: &config.Config{Hosts: hosts}}
		res := run(t, ConfigDiff{}, m, repo)
		if len(res.Diffs) != 1 || !res.Diffs[0].NoRecord || res.Diffs[0].Host != "web01" {
			t.Errorf("Diffs = %+v", res.Diffs)
		}
		if len(repo.calls) != 0 {
			t.Errorf("the repo must not be touched: %v", repo.calls)
		}
	})
	t.Run("last deployed revision", func(t *testing.T) {
		m := mock_shell.NewMockIface(gomock.NewController(t))
		m.EXPECT().Host().Return("10.0.0.1").AnyTimes()
		m.EXPECT().Execf(gomock.Any(), "", lastCmd, gomock.Any(), gomock.Any()).
			Return(*bytes.NewBufferString("abc1234\n"), bytes.Buffer{}, nil)
		m.EXPECT().Exec(gomock.Any(), "", "command -v sha256sum >/dev/null 2>&1").Return(bytes.Buffer{}, bytes.Buffer{}, nil)
		res := run(t, ConfigDiff{}, m, &fakeRepo{conf: &config.Config{Hosts: hosts}})
		if d := res.Diffs[0]; d.NoRecord || d.Revision != "abc1234" || len(d.Changes) != 0 {
			t.Errorf("Diffs = %+v", res.Diffs)
		}
	})
	t.Run("explicit revision", func(t *testing.T) {
		m := mock_shell.NewMockIface(gomock.NewController(t))
		m.EXPECT().Host().Return("10.0.0.1").AnyTimes()
		m.EXPECT().Exec(gomock.Any(), "", "command -v sha256sum >/dev/null 2>&1").Return(bytes.Buffer{}, bytes.Buffer{}, nil)
		res := run(t, ConfigDiff{GitRevision: "feature"}, m, &fakeRepo{conf: &config.Config{Hosts: hosts}})
		if d := res.Diffs[0]; d.Revision != "0123456789abcdef0123456789abcdef01234567" {
			t.Errorf("Diffs = %+v", res.Diffs)
		}
	})
}

func TestRunPush_PlaintextPasswordWarning(t *testing.T) {
	for _, plain := range []bool{true, false} {
		repo := &fakeRepo{branch: "feature", plain: plain, pushed: &localrepo.PushResult{}}
		res, err := runPush(ConfigPush{GitBranch: "feature"}, context.Background(), zaptest.NewLogger(t), repo)
		if err != nil {
			t.Fatal(err)
		}
		if got := len(res.Warnings) == 1; got != plain {
			t.Errorf("plain=%v: warnings = %v", plain, res.Warnings)
		}
	}
}

func TestRunSync_RefusesDirtyWorkingTree(t *testing.T) {
	var dirtyErr myerrors.LocalRepoDirty
	repo := &fakeRepo{branch: "master", detached: true, dirty: []string{"untracked.txt"}}
	_, err := runSync(ConfigSync{GitBranch: "master"}, context.Background(), zaptest.NewLogger(t), repo)
	if !errors.As(err, &dirtyErr) || len(dirtyErr.Paths) != 1 {
		t.Fatalf("err = %v, want LocalRepoDirty", err)
	}
	if repo.reset || len(repo.calls) != 0 {
		t.Errorf("nothing may happen to the repo: reset = %v, calls = %v", repo.reset, repo.calls)
	}
}

func TestRunSync_Force(t *testing.T) {
	repo := &fakeRepo{branch: "master", detached: true, dirty: []string{"untracked.txt"}}
	if _, err := runSync(ConfigSync{GitBranch: "master", Force: true}, context.Background(), zaptest.NewLogger(t), repo); err != nil {
		t.Fatal(err)
	}
	if !repo.reset || strings.Join(repo.calls, ",") != "merge:master" {
		t.Errorf("reset = %v, calls = %v", repo.reset, repo.calls)
	}
}

func TestRunSync_CleanDetachedResets(t *testing.T) {
	repo := &fakeRepo{branch: "master", detached: true}
	if _, err := runSync(ConfigSync{GitBranch: "master"}, context.Background(), zaptest.NewLogger(t), repo); err != nil {
		t.Fatal(err)
	}
	if !repo.reset {
		t.Error("a detached HEAD must be reset after the check")
	}
}

func TestRunDeploy_RefusesDirtyWorkingTree(t *testing.T) {
	repo := &fakeRepo{conf: &config.Config{}, dirty: []string{"web01/untracked.conf"}}
	conf := ConfigDeploy{GitRevision: "master"}
	conf.Reporter = NewRecordingReporter()
	_, err := runDeploy(conf, context.Background(), zaptest.NewLogger(t), repo, nil, nil, lockersWith(nil))
	var dirtyErr myerrors.LocalRepoDirty
	if !errors.As(err, &dirtyErr) {
		t.Fatalf("err = %v, want LocalRepoDirty", err)
	}
	if len(repo.calls) != 0 {
		t.Errorf("the working tree must not move: %v", repo.calls)
	}
}
