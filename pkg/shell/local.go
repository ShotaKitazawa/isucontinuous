package shell

import (
	"bytes"
	"context"

	"k8s.io/utils/exec"
)

type LocalClient struct {
	exec exec.Interface
}

func NewLocalClient(e exec.Interface) *LocalClient {
	return &LocalClient{e}
}

func (c *LocalClient) RunCommand(ctx context.Context, basedir string, cmd string) (bytes.Buffer, bytes.Buffer, error) {

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
