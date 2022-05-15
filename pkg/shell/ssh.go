package shell

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"os"

	"github.com/melbahja/goph"
	"golang.org/x/crypto/ssh"
)

type SshClient struct {
	*goph.Client
	host string
}

func (c *SshClient) Host() string {
	return c.host
}

func NewSshClient(host string, port int, user, password, keyfile string) (*SshClient, error) {
	var auth goph.Auth
	var err error
	switch {
	case keyfile != "":
		auth, err = goph.Key(keyfile, "")
		if err != nil {
			log.Fatal(err)
		}
	case password != "":
		auth = goph.Password(password)
	default:
		return nil, fmt.Errorf("neither password nor publicKey was specified")
	}
	conf := &goph.Config{
		User:     user,
		Addr:     host,
		Port:     uint(port),
		Auth:     auth,
		Timeout:  goph.DefaultTimeout,
		Callback: ssh.InsecureIgnoreHostKey(),
	}
	client, err := goph.NewConn(conf)
	if err != nil {
		log.Fatal(err)
	}
	return &SshClient{client, host}, nil
}

func (c *SshClient) Exec(ctx context.Context, basedir string, command string) (bytes.Buffer, bytes.Buffer, error) {
	stdout := bytes.Buffer{}
	stderr := bytes.Buffer{}
	if command == "" { // early return
		return stdout, stderr, nil
	}

	cc, err := c.CommandContext(ctx, "sh", "-c", command)
	if err != nil {
		return stdout, stderr, fmt.Errorf("Failed to create session: %v", err)
	}
	cc.Stdout = &stdout
	cc.Stderr = &stderr
	err = cc.Run()
	return trimNewLine(stdout), trimNewLine(stderr), err
}

func (c *SshClient) Execf(ctx context.Context, basedir string, cmd string, a ...interface{}) (bytes.Buffer, bytes.Buffer, error) {
	return c.Exec(ctx, basedir, fmt.Sprintf(cmd, a...))
}

func (c *SshClient) Deploy(ctx context.Context, src, dst string) error {
	s, err := os.Open(src)
	if err != nil {
		return err
	}

	sftp, err := c.NewSftp()
	if err != nil {
		return fmt.Errorf("Failed to create session: %v", err)
	}
	defer sftp.Close()

	d, err := sftp.Create(dst)
	if err != nil {
		return err
	}
	defer d.Close()

	if _, err := io.Copy(d, s); err != nil {
		return err
	}
	return nil
}
