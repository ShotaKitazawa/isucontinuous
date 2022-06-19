package deploy

import (
	"bytes"
	"context"
	"fmt"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/golang/mock/gomock"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"

	"github.com/ShotaKitazawa/isucontinuous/pkg/config"
	"github.com/ShotaKitazawa/isucontinuous/pkg/shell"
	mock_shell "github.com/ShotaKitazawa/isucontinuous/pkg/shell/mock"
	"github.com/ShotaKitazawa/isucontinuous/pkg/template"
)

func TestDeployer_Deploy(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	ctx := context.Background()
	_, testFilename, _, _ := runtime.Caller(0)
	testDir := filepath.Dir(testFilename)

	type fields struct {
		log           *zap.Logger
		shell         shell.Iface
		template      *template.Templator
		localRepoPath string
	}
	type args struct {
		targets []config.DeployTarget
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{
			name: "normal",
			fields: fields{
				log: zaptest.NewLogger(t),
				shell: func() shell.Iface {
					m := mock_shell.NewMockIface(mockCtrl)
					m.EXPECT().Host().Return("testdata")
					// /etc/nginx/nginx.conf (/etc/nginx is existed)
					m.EXPECT().Execf(ctx, "", `test -d "%s"`, "/etc/nginx").
						Return(bytes.Buffer{}, bytes.Buffer{}, nil)
					m.EXPECT().Deploy(ctx, filepath.Join(testDir, "testdata", "nginx/nginx.conf"), "/etc/nginx/nginx.conf").
						Return(nil)
					// /etc/nginx/sites-available/default (/etc/nginxsites-available isn't existed)
					m.EXPECT().Execf(ctx, "", `test -d "%s"`, "/etc/nginx/sites-available").
						Return(bytes.Buffer{}, bytes.Buffer{}, fmt.Errorf(""))
					m.EXPECT().Execf(ctx, "", `mkdir -p "%s"`, "/etc/nginx/sites-available").
						Return(bytes.Buffer{}, bytes.Buffer{}, nil)
					m.EXPECT().Deploy(ctx, filepath.Join(testDir, "testdata", "nginx/sites-available/default"), "/etc/nginx/sites-available/default").
						Return(nil)
					// /etc/nginx/sites-available/default (/etc/nginxsites-available is existed)
					m.EXPECT().Execf(ctx, "", `test -d "%s"`, "/etc/nginx/sites-available").
						Return(bytes.Buffer{}, bytes.Buffer{}, nil)
					m.EXPECT().Deploy(ctx, filepath.Join(testDir, "testdata", "nginx/sites-available/isucondition.conf"), "/etc/nginx/sites-available/isucondition.conf").
						Return(nil)
					// /etc/nginx/sites-enabled/isucondition.conf (/etc/nginxsites-available is existed)
					m.EXPECT().Execf(ctx, "", `test -d "%s"`, "/etc/nginx/sites-enabled").
						Return(bytes.Buffer{}, bytes.Buffer{}, nil)
					{ // recursive due to resolve symlink
						m.EXPECT().Host().Return("testdata")
						// /etc/nginx/sites-available/default (/etc/nginxsites-available is existed)
						m.EXPECT().Execf(ctx, "", `test -d "%s"`, "/etc/nginx/sites-enabled").
							Return(bytes.Buffer{}, bytes.Buffer{}, nil)
						m.EXPECT().Deploy(ctx, filepath.Join(testDir, "testdata", "nginx/sites-available/isucondition.conf"), "/etc/nginx/sites-enabled/isucondition.conf").
							Return(nil)
					}
					return m
				}(),
			},
			args: args{targets: []config.DeployTarget{
				{
					Src:    "nginx",
					Target: "/etc/nginx",
				},
			}},
		},
		{
			name: "normal_topLevelFileIsSymlink",
			fields: fields{
				log: zaptest.NewLogger(t),
				shell: func() shell.Iface {
					m := mock_shell.NewMockIface(mockCtrl)
					m.EXPECT().Host().Return("testdata")
					m.EXPECT().Execf(ctx, "", `test -d "%s"`, "/etc").
						Return(bytes.Buffer{}, bytes.Buffer{}, nil)
					{ // recursive due to resolve symlink
						m.EXPECT().Host().Return("testdata")
						// /etc/nginx/nginx.conf (/etc/nginx is existed)
						m.EXPECT().Execf(ctx, "", `test -d "%s"`, "/etc/nginx").
							Return(bytes.Buffer{}, bytes.Buffer{}, nil)
						m.EXPECT().Deploy(ctx, filepath.Join(testDir, "testdata", "nginx/nginx.conf"), "/etc/nginx/nginx.conf").
							Return(nil)
						// /etc/nginx/sites-available/default (/etc/nginxsites-available isn't existed)
						m.EXPECT().Execf(ctx, "", `test -d "%s"`, "/etc/nginx/sites-available").
							Return(bytes.Buffer{}, bytes.Buffer{}, fmt.Errorf(""))
						m.EXPECT().Execf(ctx, "", `mkdir -p "%s"`, "/etc/nginx/sites-available").
							Return(bytes.Buffer{}, bytes.Buffer{}, nil)
						m.EXPECT().Deploy(ctx, filepath.Join(testDir, "testdata", "nginx/sites-available/default"), "/etc/nginx/sites-available/default").
							Return(nil)
						// /etc/nginx/sites-available/default (/etc/nginxsites-available is existed)
						m.EXPECT().Execf(ctx, "", `test -d "%s"`, "/etc/nginx/sites-available").
							Return(bytes.Buffer{}, bytes.Buffer{}, nil)
						m.EXPECT().Deploy(ctx, filepath.Join(testDir, "testdata", "nginx/sites-available/isucondition.conf"), "/etc/nginx/sites-available/isucondition.conf").
							Return(nil)
						// /etc/nginx/sites-enabled/isucondition.conf (/etc/nginxsites-available is existed)
						m.EXPECT().Execf(ctx, "", `test -d "%s"`, "/etc/nginx/sites-enabled").
							Return(bytes.Buffer{}, bytes.Buffer{}, nil)
						{ // recursive due to resolve symlink
							m.EXPECT().Host().Return("testdata")
							// /etc/nginx/sites-available/default (/etc/nginxsites-available is existed)
							m.EXPECT().Execf(ctx, "", `test -d "%s"`, "/etc/nginx/sites-enabled").
								Return(bytes.Buffer{}, bytes.Buffer{}, nil)
							m.EXPECT().Deploy(ctx, filepath.Join(testDir, "testdata", "nginx/sites-available/isucondition.conf"), "/etc/nginx/sites-enabled/isucondition.conf").
								Return(nil)
						}
					}
					return m
				}(),
			},
			args: args{targets: []config.DeployTarget{
				{
					Src:    "nginx_symlink",
					Target: "/etc/nginx",
				},
			}},
		},
		{
			name: "abnormal_symlinkCannotResolve",
			fields: fields{
				log: zaptest.NewLogger(t),
				shell: func() shell.Iface {
					m := mock_shell.NewMockIface(mockCtrl)
					m.EXPECT().Host().Return("testdata")
					m.EXPECT().Execf(ctx, "", `test -d "%s"`, "/etc/error").
						Return(bytes.Buffer{}, bytes.Buffer{}, nil)
					return m
				}(),
			},
			args: args{targets: []config.DeployTarget{
				{
					Src:    "error",
					Target: "/etc/error",
				},
			}},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := Deployer{
				log:           tt.fields.log,
				shell:         tt.fields.shell,
				template:      tt.fields.template,
				localRepoPath: testDir,
			}
			if err := d.Deploy(ctx, tt.args.targets); (err != nil) != tt.wantErr {
				t.Errorf("Deployer.Deploy() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
