package shell

import (
	"bytes"
	"context"
	"fmt"
	"os"

	"golang.org/x/crypto/ssh"
)

type SshClient struct {
	ssh.ClientConfig
	target string
}

func (c *SshClient) Host() string {
	return c.target
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
	// Connect to the remote server and perform the SSH handshake.
	target := fmt.Sprintf("%s:%d", host, port)
	if port == 0 {
		target = fmt.Sprintf("%s:22", host)
	}
	if _, err := ssh.Dial("tcp", target, &config); err != nil {
		return nil, fmt.Errorf("unable to connect: %v", err)
	}
	return &SshClient{config, target}, nil
}

func (c *SshClient) RunCommand(ctx context.Context, basedir string, command string) (bytes.Buffer, bytes.Buffer, error) {
	stdout := bytes.Buffer{}
	stderr := bytes.Buffer{}

	client, err := ssh.Dial("tcp", c.target, &c.ClientConfig)
	if err != nil {
		return stdout, stderr, fmt.Errorf("unable to connect: %v", err)
	}
	defer client.Close()
	session, err := client.NewSession()
	if err != nil {
		return stdout, stderr, fmt.Errorf("Failed to create session: %v", err)
	}
	defer session.Close()

	session.Stdout = &stdout
	session.Stderr = &stderr
	if basedir != "" {
		command = "cd " + basedir + "; " + command
	}
	if err := session.Run(command); err != nil {
		return stdout, stderr, err
	}

	return stdout, stderr, nil
}
