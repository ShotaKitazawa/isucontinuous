package deploy

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"

	"github.com/golang/mock/gomock"
	"go.uber.org/zap/zaptest"

	"github.com/ShotaKitazawa/isucontinuous/pkg/config"
	mock_shell "github.com/ShotaKitazawa/isucontinuous/pkg/shell/mock"
)

func testdataDir() string {
	_, testFilename, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(testFilename), "testdata")
}

func sumLine(t *testing.T, src, dst string) string {
	t.Helper()
	b, err := os.ReadFile(src)
	if err != nil {
		t.Fatal(err)
	}
	return fmt.Sprintf("%x  %s\n", sha256.Sum256(b), dst)
}

func TestDeployer_Plan(t *testing.T) {
	testDir := testdataDir()
	nginxFiles := func(dstRoot string) []PlanFile {
		return []PlanFile{
			{filepath.Join(testDir, "host01/nginx/nginx.conf"), dstRoot + "/nginx.conf", "host01/nginx/nginx.conf"},
			{filepath.Join(testDir, "host01/nginx/sites-available/default"), dstRoot + "/sites-available/default", "host01/nginx/sites-available/default"},
			{filepath.Join(testDir, "host01/nginx/sites-available/isucondition.conf"), dstRoot + "/sites-available/isucondition.conf", "host01/nginx/sites-available/isucondition.conf"},
			{filepath.Join(testDir, "host01/nginx/sites-available/isucondition.conf"), dstRoot + "/sites-enabled/isucondition.conf", "host01/nginx/sites-available/isucondition.conf"},
		}
	}
	tests := []struct {
		name    string
		host    string
		targets []config.DeployTarget
		want    []TargetPlan
		wantErr bool
	}{
		{
			name:    "normal",
			host:    "host01",
			targets: []config.DeployTarget{{Src: "nginx", Target: "/etc/nginx", Compile: "make"}},
			want:    []TargetPlan{{Files: nginxFiles("/etc/nginx"), Compile: "make", CompileDir: "/etc/nginx"}},
		},
		{
			name:    "normal_symlinkToSameHost",
			host:    "host01",
			targets: []config.DeployTarget{{Src: "nginx_symlink", Target: "/etc/nginx"}},
			want:    []TargetPlan{{Files: nginxFiles("/etc/nginx"), CompileDir: "/etc/nginx"}},
		},
		{
			name:    "normal_symlinkToOtherHost",
			host:    "host02",
			targets: []config.DeployTarget{{Src: "nginx_symlink", Target: "/etc/nginx"}},
			want:    []TargetPlan{{Files: nginxFiles("/etc/nginx"), CompileDir: "/etc/nginx"}},
		},
		{
			name:    "abnormal_symlinkCannotResolve",
			host:    "host01",
			targets: []config.DeployTarget{{Src: "error", Target: "/etc/error"}},
			wantErr: true,
		},
		{
			name:    "abnormal_symlinkToOutsideOfLocalRepo",
			host:    "host02",
			targets: []config.DeployTarget{{Src: "hosts_symlink", Target: "/etc/hosts"}},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := mock_shell.NewMockIface(gomock.NewController(t))
			m.EXPECT().Host().Return("server").AnyTimes()
			d := Deployer{log: zaptest.NewLogger(t), shell: m, localRepoPath: testDir}
			plan, err := d.Plan(context.Background(), tt.host, tt.targets)
			if (err != nil) != tt.wantErr {
				t.Fatalf("Plan() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err == nil && !reflect.DeepEqual(plan.Targets, tt.want) {
				t.Errorf("Plan() = %+v, want %+v", plan.Targets, tt.want)
			}
		})
	}
}

func TestDeployer_Execute(t *testing.T) {
	ctx := context.Background()
	testDir := testdataDir()
	nginxConf := filepath.Join(testDir, "host01/nginx/nginx.conf")
	defaultSite := filepath.Join(testDir, "host01/nginx/sites-available/default")
	plan := &Plan{Targets: []TargetPlan{{
		Files: []PlanFile{
			{nginxConf, "/etc/nginx/nginx.conf", "host01/nginx/nginx.conf"},
			{defaultSite, "/etc/nginx/sites-available/default", "host01/nginx/sites-available/default"},
		},
		Compile: "make", CompileDir: "/etc/nginx",
	}}}
	ok := func() (bytes.Buffer, bytes.Buffer, error) { return bytes.Buffer{}, bytes.Buffer{}, nil }
	const sumCmd = "sha256sum -- %s 2>/dev/null || true"
	const sumArgs = `'/etc/nginx/nginx.conf' '/etc/nginx/sites-available/default'`

	t.Run("transfers only changed files", func(t *testing.T) {
		m := mock_shell.NewMockIface(gomock.NewController(t))
		m.EXPECT().Host().Return("server").AnyTimes()
		m.EXPECT().Exec(ctx, "", "command -v sha256sum >/dev/null 2>&1").Return(ok())
		// nginx.conf is unchanged; sites-available/default is absent on the host
		m.EXPECT().Execf(ctx, "", sumCmd, sumArgs).
			Return(*bytes.NewBufferString(sumLine(t, nginxConf, "/etc/nginx/nginx.conf")), bytes.Buffer{}, nil)
		m.EXPECT().Execf(ctx, "", "mkdir -p %s", `'/etc/nginx/sites-available'`).Return(ok())
		m.EXPECT().Deploy(ctx, defaultSite, "/etc/nginx/sites-available/default").Return(nil)
		m.EXPECT().Exec(ctx, "/etc/nginx", "make").Return(ok())
		d := Deployer{log: zaptest.NewLogger(t), shell: m, localRepoPath: testDir}
		if err := d.Execute(ctx, "host01", plan); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("transfers everything without sha256sum", func(t *testing.T) {
		m := mock_shell.NewMockIface(gomock.NewController(t))
		m.EXPECT().Host().Return("server").AnyTimes()
		m.EXPECT().Exec(ctx, "", "command -v sha256sum >/dev/null 2>&1").
			Return(bytes.Buffer{}, bytes.Buffer{}, errors.New("exit 1"))
		m.EXPECT().Execf(ctx, "", "mkdir -p %s", `'/etc/nginx' '/etc/nginx/sites-available'`).Return(ok())
		m.EXPECT().Deploy(ctx, nginxConf, "/etc/nginx/nginx.conf").Return(nil)
		m.EXPECT().Deploy(ctx, defaultSite, "/etc/nginx/sites-available/default").Return(nil)
		m.EXPECT().Exec(ctx, "/etc/nginx", "make").Return(ok())
		d := Deployer{log: zaptest.NewLogger(t), shell: m, localRepoPath: testDir}
		if err := d.Execute(ctx, "host01", plan); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("compile runs even if nothing changed", func(t *testing.T) {
		m := mock_shell.NewMockIface(gomock.NewController(t))
		m.EXPECT().Host().Return("server").AnyTimes()
		m.EXPECT().Exec(ctx, "", "command -v sha256sum >/dev/null 2>&1").Return(ok())
		m.EXPECT().Execf(ctx, "", sumCmd, sumArgs).Return(*bytes.NewBufferString(
			sumLine(t, nginxConf, "/etc/nginx/nginx.conf") + sumLine(t, defaultSite, "/etc/nginx/sites-available/default")),
			bytes.Buffer{}, nil)
		m.EXPECT().Exec(ctx, "/etc/nginx", "make").Return(ok())
		d := Deployer{log: zaptest.NewLogger(t), shell: m, localRepoPath: testDir}
		if err := d.Execute(ctx, "host01", plan); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("compile failure", func(t *testing.T) {
		m := mock_shell.NewMockIface(gomock.NewController(t))
		m.EXPECT().Host().Return("server").AnyTimes()
		m.EXPECT().Exec(ctx, "", gomock.Any()).Return(bytes.Buffer{}, bytes.Buffer{}, errors.New("x"))
		m.EXPECT().Execf(ctx, "", "mkdir -p %s", gomock.Any()).Return(ok())
		m.EXPECT().Deploy(ctx, gomock.Any(), gomock.Any()).Return(nil).Times(2)
		m.EXPECT().Exec(ctx, "/etc/nginx", "make").Return(bytes.Buffer{}, *bytes.NewBufferString("boom"), errors.New("exit 2"))
		d := Deployer{log: zaptest.NewLogger(t), shell: m, localRepoPath: testDir}
		if err := d.Execute(ctx, "host01", plan); err == nil {
			t.Fatal("want error")
		}
	})
}

func TestNewDeployers_KeyedByName(t *testing.T) {
	deployers, err := NewDeployers(nil, nil, "", []config.Host{{Name: "web01", Host: "10.0.0.1", Local: true}})
	if err != nil {
		t.Fatal(err)
	}
	if deployers["web01"] == nil {
		t.Errorf("deployer must be keyed by host.Name: %v", deployers)
	}
}
