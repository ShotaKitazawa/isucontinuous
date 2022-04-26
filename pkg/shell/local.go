package shell

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"

	"k8s.io/utils/exec"
)

type LocalClient struct {
	exec exec.Interface
}

func NewLocalClient(e exec.Interface) *LocalClient {
	return &LocalClient{e}
}

func (c *LocalClient) Host() string {
	return "localhost"
}

func (c *LocalClient) Exec(ctx context.Context, basedir string, cmd string) (bytes.Buffer, bytes.Buffer, error) {
	stdout := bytes.Buffer{}
	stderr := bytes.Buffer{}
	cc := c.exec.CommandContext(ctx, "sh", "-c", cmd)
	if basedir != "" {
		cc.SetDir(basedir)
	}
	cc.SetStdout(&stdout)
	cc.SetStderr(&stderr)
	if err := cc.Run(); err != nil {
		return stdout, stderr, err
	}
	return stdout, stderr, nil
}

func (c *LocalClient) Execf(ctx context.Context, basedir string, cmd string, a ...string) (bytes.Buffer, bytes.Buffer, error) {
	return c.Exec(ctx, basedir, fmt.Sprintf(cmd, a))
}

func (c *LocalClient) Deploy(ctx context.Context, src, dst string) error {
	s, err := os.Open(src)
	if err != nil {
		return err
	}
	d, err := os.Create(dst)
	if err != nil {
		return err
	}
	if _, err := io.Copy(d, s); err != nil {
		return err
	}
	return nil
}
