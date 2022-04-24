package shell

import (
	"bytes"
	"context"
)

type Iface interface {
	Host() string
	RunCommand(ctx context.Context, basedir string, command string) (bytes.Buffer, bytes.Buffer, error)
}
