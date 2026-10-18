package install

import (
	"bytes"
	"context"
	"errors"
	"os"
	"reflect"
	"testing"

	"go.uber.org/zap"
)

type alpShell struct {
	installed bool
	failCurl  bool
	commands  []string
}

func (s *alpShell) Host() string { return "web01" }

func (s *alpShell) Exec(_ context.Context, _, command string) (bytes.Buffer, bytes.Buffer, error) {
	s.commands = append(s.commands, command)
	if command == "which -a alp" && s.installed {
		return *bytes.NewBufferString("/usr/local/bin/alp"), bytes.Buffer{}, nil
	}
	if s.failCurl && len(command) >= 4 && command[:4] == "curl" {
		return bytes.Buffer{}, *bytes.NewBufferString("download failed"), errors.New("exit status 22")
	}
	return bytes.Buffer{}, bytes.Buffer{}, nil
}

func (s *alpShell) Execf(ctx context.Context, basedir, command string, args ...interface{}) (bytes.Buffer, bytes.Buffer, error) {
	return s.Exec(ctx, basedir, command)
}

func (s *alpShell) Deploy(context.Context, string, string) error { return nil }

func (s *alpShell) ReadFile(context.Context, string) ([]byte, error) { return nil, os.ErrNotExist }

func TestAlpDownloadURL(t *testing.T) {
	tests := []struct {
		name    string
		version string
		url     string
	}{
		{"latest", "latest", "https://github.com/tkuchiki/alp/releases/latest/download/alp_linux_amd64.zip"},
		{"explicit version", "v1.0.21", "https://github.com/tkuchiki/alp/releases/download/v1.0.21/alp_linux_amd64.zip"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &alpShell{}
			if err := new(zap.NewNop(), s).Alp(context.Background(), tt.version); err != nil {
				t.Fatal(err)
			}
			want := []string{
				"which -a alp",
				"curl -fSL '" + tt.url + "' -o /tmp/alp.zip",
				"unzip /tmp/alp.zip -d /usr/local/bin/",
			}
			if !reflect.DeepEqual(s.commands, want) {
				t.Errorf("commands = %#v, want %#v", s.commands, want)
			}
		})
	}
}

func TestAlpCurlFailurePreventsUnzip(t *testing.T) {
	s := &alpShell{failCurl: true}
	if err := new(zap.NewNop(), s).Alp(context.Background(), "latest"); err == nil {
		t.Fatal("want download error")
	}
	if len(s.commands) != 2 {
		t.Fatalf("commands = %#v; unzip must not run", s.commands)
	}
}

func TestAlpAlreadyInstalledSkipsDownload(t *testing.T) {
	s := &alpShell{installed: true}
	if err := new(zap.NewNop(), s).Alp(context.Background(), "latest"); err != nil {
		t.Fatal(err)
	}
	if want := []string{"which -a alp"}; !reflect.DeepEqual(s.commands, want) {
		t.Errorf("commands = %#v, want %#v", s.commands, want)
	}
}
