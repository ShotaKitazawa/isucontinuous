package cmd

import (
	"context"
	"errors"
	"io/ioutil"
	"os"
	"strings"
	"testing"

	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"

	"github.com/ShotaKitazawa/isucontinuous/pkg/config"
	"github.com/ShotaKitazawa/isucontinuous/pkg/localrepo"
	"github.com/ShotaKitazawa/isucontinuous/pkg/slack"
	"github.com/ShotaKitazawa/isucontinuous/pkg/template"
	"github.com/ShotaKitazawa/isucontinuous/pkg/usecases/deploy"
)

type fakeRepo struct {
	localrepo.LocalRepoIface
	branch   string
	revision string
	pushed   *localrepo.PushResult
	conf     *config.Config
	disabled bool
	calls    []string
}

func (f *fakeRepo) CurrentBranch(ctx context.Context) (string, error) { return f.branch, nil }
func (f *fakeRepo) Fetch(ctx context.Context) error                   { return nil }
func (f *fakeRepo) DiffWithRemote(ctx context.Context) (bool, error)  { return true, nil }
func (f *fakeRepo) Push(ctx context.Context, m string) (*localrepo.PushResult, error) {
	return f.pushed, nil
}
func (f *fakeRepo) LoadConf() (*config.Config, error) { return f.conf, nil }
func (f *fakeRepo) HasDisabledHost() (bool, error)    { return f.disabled, nil }
func (f *fakeRepo) SwitchDetachedBranch(ctx context.Context, r string) error {
	f.calls = append(f.calls, "switch")
	return nil
}
func (f *fakeRepo) GetRevision(ctx context.Context) (string, error) {
	if f.revision == "" {
		return "", errors.New("not found")
	}
	return f.revision, nil
}
func (f *fakeRepo) SetRevision(ctx context.Context, r string) error {
	f.revision = r
	return nil
}
func (f *fakeRepo) GetHeadInfo(ctx context.Context) (string, string, error) {
	return "abc123", "message", nil
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
	repo := &fakeRepo{conf: &config.Config{}}
	conf := ConfigDeploy{GitRevision: "master"}
	conf.Reporter = NewRecordingReporter()
	newDeployers := func(*zap.Logger, *template.Templator, string, []config.Host) (map[string]*deploy.Deployer, error) {
		return map[string]*deploy.Deployer{}, nil
	}
	newSlackClient := func(*config.Config) slack.ClientIface {
		repo.calls = append(repo.calls, "slack")
		return slack.NewFakeClient(zap.NewNop())
	}
	var res *DeployResult
	var err error
	assertNoStdout(t, func() {
		res, err = runDeploy(conf, context.Background(), zaptest.NewLogger(t), repo, newSlackClient, newDeployers)
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Hash != "abc123" || res.Message != "message" || res.Revision != "master" {
		t.Errorf("result = %+v", res)
	}
	if repo.revision != "master" {
		t.Errorf("revision = %q, want master", repo.revision)
	}
	if want := "switch,slack"; strings.Join(repo.calls, ",") != want {
		t.Errorf("calls = %v, want %s", repo.calls, want)
	}
}
