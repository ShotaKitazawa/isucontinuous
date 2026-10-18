package shell

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

type SshClient struct {
	*ssh.Client
	host   string
	target string

	sftpMu sync.Mutex
	sftp   *sftp.Client
}

const sshSignalGracePeriod = time.Second

type sshSession interface {
	Start(string) error
	Wait() error
	Signal(ssh.Signal) error
	Close() error
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
			HostKeyCallback: tofuHostKeyCallback(),
			Timeout:         sshConnectTimeout * time.Second,
		}
	case password != "":
		config = ssh.ClientConfig{
			User: user,
			Auth: []ssh.AuthMethod{
				ssh.Password(password),
			},
			HostKeyCallback: tofuHostKeyCallback(),
			Timeout:         sshConnectTimeout * time.Second,
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
	return &SshClient{Client: conn, host: host, target: target}, nil
}

func (c *SshClient) Exec(ctx context.Context, basedir string, command string) (bytes.Buffer, bytes.Buffer, error) {
	stdout := bytes.Buffer{}
	stderr := bytes.Buffer{}
	if command == "" { // early return
		return stdout, stderr, nil
	}

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
	err = runSSHSession(ctx, session, command)
	return trimNewLine(stdout), trimNewLine(stderr), err
}

func runSSHSession(ctx context.Context, session sshSession, command string) error {
	return runSSHSessionWithGracePeriod(ctx, session, command, sshSignalGracePeriod)
}

func runSSHSessionWithGracePeriod(ctx context.Context, session sshSession, command string, gracePeriod time.Duration) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := session.Start(command); err != nil {
		return err
	}
	wait := make(chan error, 1)
	go func() { wait <- session.Wait() }()

	select {
	case err := <-wait:
		return err
	case <-ctx.Done():
	}

	_ = session.Signal(ssh.SIGINT)
	if waitForSSHSession(wait, gracePeriod) {
		return ctx.Err()
	}
	_ = session.Signal(ssh.SIGTERM)
	if waitForSSHSession(wait, gracePeriod) {
		return ctx.Err()
	}
	_ = session.Close()
	return ctx.Err()
}

func waitForSSHSession(wait <-chan error, gracePeriod time.Duration) bool {
	timer := time.NewTimer(gracePeriod)
	defer timer.Stop()
	select {
	case <-wait:
		return true
	case <-timer.C:
		return false
	}
}

func (c *SshClient) Execf(ctx context.Context, basedir string, cmd string, a ...interface{}) (bytes.Buffer, bytes.Buffer, error) {
	return c.Exec(ctx, basedir, fmt.Sprintf(cmd, a...))
}

// sftpClient is created once per host because opening a subsystem per file costs a round trip over WAN.
func (c *SshClient) sftpClient() (*sftp.Client, error) {
	c.sftpMu.Lock()
	defer c.sftpMu.Unlock()
	if c.sftp != nil {
		return c.sftp, nil
	}
	client, err := sftp.NewClient(c.Client)
	if err != nil {
		return nil, fmt.Errorf("Failed to create session: %v", err)
	}
	c.sftp = client
	return client, nil
}

func (c *SshClient) Deploy(ctx context.Context, src, dst string) error {
	s, err := os.Open(src)
	if err != nil {
		return err
	}
	defer s.Close()

	client, err := c.sftpClient()
	if err != nil {
		return err
	}
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

func (c *SshClient) ReadFile(ctx context.Context, path string) ([]byte, error) {
	client, err := c.sftpClient()
	if err != nil {
		return nil, err
	}
	f, err := client.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return io.ReadAll(f)
}
