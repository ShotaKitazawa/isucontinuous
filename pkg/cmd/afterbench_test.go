package cmd

import (
	"bytes"
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"

	"github.com/ShotaKitazawa/isucontinuous/pkg/config"
	"github.com/ShotaKitazawa/isucontinuous/pkg/hoststate"
	"github.com/ShotaKitazawa/isucontinuous/pkg/slack"
	"github.com/ShotaKitazawa/isucontinuous/pkg/template"
	"github.com/ShotaKitazawa/isucontinuous/pkg/usecases/afterbench"
)

// outputShell serves files under a directory in addition to what FakeShell understands.
type outputShell struct {
	*hoststate.FakeShell
	files map[string]string
	cats  []string
}

func (s *outputShell) Execf(ctx context.Context, basedir, command string, a ...interface{}) (bytes.Buffer, bytes.Buffer, error) {
	switch command {
	case `find "%s" -type f`:
		var names []string
		for name := range s.files {
			if strings.HasPrefix(name, a[0].(string)+"/") {
				names = append(names, name)
			}
		}
		// a trailing newline, as find prints
		return *bytes.NewBufferString(strings.Join(names, "\n") + "\n"), bytes.Buffer{}, nil
	case "cat %s":
		s.cats = append(s.cats, a[0].(string))
		return *bytes.NewBufferString(s.files[a[0].(string)]), bytes.Buffer{}, nil
	}
	return s.FakeShell.Execf(ctx, basedir, command, a...)
}

func (e *lockEnv) afterbenchWithOutput(files map[string]string, slackClient slack.ClientIface) (*AfterBenchResult, *outputShell, error) {
	e.hosts = e.hosts[:1]
	e.repo.conf = &config.Config{Slack: config.Slack{DefaultChannelId: "C123"}, Hosts: e.hosts}
	e.lock("web01", me, "deploy")
	out := &outputShell{FakeShell: e.shells["web01"], files: files}
	newAfterBenchers := func(l *zap.Logger, t *template.Templator, s slack.ClientIface, host config.Host) (*afterbench.AfterBencher, error) {
		return afterbench.NewWithShell(l, out, t, s), nil
	}
	res, err := runAfterBench(ConfigAfterBench{ConfigCommon: e.common()}, context.Background(), zaptest.NewLogger(e.t), e.repo,
		func(*config.Config) (slack.ClientIface, error) { return slackClient, nil }, newAfterBenchers,
		lockersWith(map[string]*hoststate.FakeShell{"web01": e.shells["web01"]}))
	return res, out, err
}

func TestRunAfterBench_Artifacts(t *testing.T) {
	files := map[string]string{"/p/abc1234/alp.txt": "alp output\n"}
	t.Run("the result carries the content read once and the path after the move", func(t *testing.T) {
		e := newLockEnv(t)
		res, out, err := e.afterbenchWithOutput(files, slack.NewFakeClient(zap.NewNop()))
		if err != nil {
			t.Fatal(err)
		}
		if len(res.Artifacts) != 1 {
			t.Fatalf("artifacts = %+v", res.Artifacts)
		}
		a := res.Artifacts[0]
		if a.Host != "web01" || a.Content != "alp output\n" || a.Size != len("alp output\n") || a.Note != "" {
			t.Errorf("artifact = %+v", a)
		}
		if !strings.HasPrefix(a.Path, "/p/abc1234.") || !strings.HasSuffix(a.Path, "/alp.txt") {
			t.Errorf("path = %q, want a path under the moved directory", a.Path)
		}
		if !reflect.DeepEqual(out.cats, []string{"/p/abc1234/alp.txt"}) {
			t.Errorf("cat ran %v, want once for the file", out.cats)
		}
	})
	t.Run("a failed send to Slack is a warning and the artifacts remain", func(t *testing.T) {
		e := newLockEnv(t)
		res, _, err := e.afterbenchWithOutput(files, slack.NewFailingClient(errors.New("not_in_channel")))
		if err != nil {
			t.Fatal(err)
		}
		if len(res.Artifacts) != 1 || len(res.Warnings) != 1 || !strings.Contains(res.Warnings[0], "not_in_channel") {
			t.Errorf("artifacts = %+v, warnings = %v", res.Artifacts, res.Warnings)
		}
	})
}
