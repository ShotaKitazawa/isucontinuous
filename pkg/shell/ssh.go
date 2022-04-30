package shell

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

type SshClient struct {
	*ssh.Client
	host   string
	target string
}

func (c *SshClient) Host() string {
	return c.host
}

func NewSshClient(host string, port int, user, password, keyfile string) (*SshClient, error) {
	var config ssh.ClientConfig
	switch {
	case keyfile != "":
		key, err := os.ReadFile(keyfile)
		if err != nil {
			return nil, fmt.Errorf("unable to read private key: %v", err)
		}
		signer, err := ssh.ParsePrivateKey(key)
		if err != nil {
			return nil, fmt.Errorf("unable to parse private key: %v", err)
		}
		config = ssh.ClientConfig{
			User: user,
			Auth: []ssh.AuthMethod{
				ssh.PublicKeys(signer),
			},
			HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		}
	case password != "":
		config = ssh.ClientConfig{
			User: user,
			Auth: []ssh.AuthMethod{
				ssh.Password(password),
			},
			HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		}
	default:
		return nil, fmt.Errorf("neither password nor publicKey was specified")
	}
	config.SetDefaults()
	// Connect to the remote server and perform the SSH handshake.
	target := fmt.Sprintf("%s:%d", host, port)
	if port == 0 {
		target = fmt.Sprintf("%s:22", host)
	}
	conn, err := ssh.Dial("tcp", target, &config)
	if err != nil {
		return nil, fmt.Errorf("unable to connect: %v", err)
	}
	return &SshClient{conn, host, target}, nil
}

func (c *SshClient) Exec(ctx context.Context, basedir string, command string) (bytes.Buffer, bytes.Buffer, error) {
	stdout := bytes.Buffer{}
	stderr := bytes.Buffer{}

	session, err := c.NewSession()
	if err != nil {
		return stdout, stderr, fmt.Errorf("Failed to create session: %v", err)
	}
	defer session.Close()

	session.Stdout = &stdout
	session.Stderr = &stderr
	if basedir != "" {
		command = "cd " + basedir + "; " + command
	}
	err = session.Run(command)
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

	client, err := sftp.NewClient(c.Client)
	if err != nil {
		return fmt.Errorf("Failed to create session: %v", err)
	}
	defer client.Close()

	d, err := client.Create(dst)
	if err != nil {
		return err
	}
	defer d.Close()

	if _, err := io.Copy(d, s); err != nil {
		return err
	}
	return nil
}
