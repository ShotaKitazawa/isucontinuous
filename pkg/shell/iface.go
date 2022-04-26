package shell

import (
	"bytes"
	"context"
)

type Iface interface {
	Host() string
	Exec(ctx context.Context, basedir string, command string) (bytes.Buffer, bytes.Buffer, error)
	Execf(ctx context.Context, basedir string, command string, a ...string) (bytes.Buffer, bytes.Buffer, error)
	Deploy(ctx context.Context, src, dst string) error
}
