package hoststate

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/golang/mock/gomock"

	mock_shell "github.com/ShotaKitazawa/isucontinuous/pkg/shell/mock"
)

const hash = "0123456789abcdef0123456789abcdef01234567"

func TestWriteLastRevision(t *testing.T) {
	ctx := context.Background()
	write := `mkdir -p %s && printf '%%s\n' %s > %s`

	t.Run("primary", func(t *testing.T) {
		m := mock_shell.NewMockIface(gomock.NewController(t))
		m.EXPECT().Execf(ctx, "", write, "'/var/lib/isucontinuous'", hash, "'/var/lib/isucontinuous/last-revision'").
			Return(bytes.Buffer{}, bytes.Buffer{}, nil)
		m.EXPECT().Execf(ctx, "", `rm -f %s`, "'/tmp/isucontinuous/last-revision'").
			Return(bytes.Buffer{}, bytes.Buffer{}, nil)
		if err := WriteLastRevision(ctx, m, hash); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("fallback", func(t *testing.T) {
		m := mock_shell.NewMockIface(gomock.NewController(t))
		m.EXPECT().Execf(ctx, "", write, "'/var/lib/isucontinuous'", hash, "'/var/lib/isucontinuous/last-revision'").
			Return(bytes.Buffer{}, bytes.Buffer{}, errors.New("denied"))
		m.EXPECT().Execf(ctx, "", write, "'/tmp/isucontinuous'", hash, "'/tmp/isucontinuous/last-revision'").
			Return(bytes.Buffer{}, bytes.Buffer{}, nil)
		if err := WriteLastRevision(ctx, m, hash); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("both fail", func(t *testing.T) {
		m := mock_shell.NewMockIface(gomock.NewController(t))
		m.EXPECT().Execf(ctx, "", write, gomock.Any(), hash, gomock.Any()).
			Return(bytes.Buffer{}, bytes.Buffer{}, errors.New("denied")).Times(2)
		if err := WriteLastRevision(ctx, m, hash); err == nil {
			t.Fatal("want error")
		}
	})
	t.Run("invalid hash", func(t *testing.T) {
		m := mock_shell.NewMockIface(gomock.NewController(t))
		if err := WriteLastRevision(ctx, m, "x; rm -rf /"); err == nil {
			t.Fatal("want error")
		}
	})
}

func TestReadLastRevision(t *testing.T) {
	ctx := context.Background()
	read := `cat %s 2>/dev/null || cat %s 2>/dev/null || true`
	tests := []struct {
		name      string
		out       string
		wantHash  string
		wantFound bool
		wantErr   bool
	}{
		{name: "recorded", out: hash + "\n", wantHash: hash, wantFound: true},
		{name: "not recorded", out: ""},
		{name: "corrupted", out: "garbage", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := mock_shell.NewMockIface(gomock.NewController(t))
			m.EXPECT().Execf(ctx, "", read, "'/var/lib/isucontinuous/last-revision'", "'/tmp/isucontinuous/last-revision'").
				Return(*bytes.NewBufferString(tt.out), bytes.Buffer{}, nil)
			h, found, err := ReadLastRevision(ctx, m)
			if (err != nil) != tt.wantErr || h != tt.wantHash || found != tt.wantFound {
				t.Errorf("got (%q, %v, %v)", h, found, err)
			}
		})
	}
}
