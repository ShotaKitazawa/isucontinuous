package shell

import (
	"bytes"
	"context"
)

type Iface interface {
	Host() string
	Exec(ctx context.Context, basedir string, command string) (bytes.Buffer, bytes.Buffer, error)
	Execf(ctx context.Context, basedir string, command string, a ...interface{}) (bytes.Buffer, bytes.Buffer, error)
	Deploy(ctx context.Context, src, dst string) error
}

func trimNewLine(buf bytes.Buffer) bytes.Buffer {
	b := buf.Bytes()
	b = bytes.TrimRight(b, "\n")
	return *bytes.NewBuffer(b)
}
