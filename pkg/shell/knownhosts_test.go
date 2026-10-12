package shell

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/crypto/ssh"
)

func newSigner(t *testing.T) ssh.Signer {
	t.Helper()
	k, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	s, err := ssh.NewSignerFromKey(k)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func startServer(t *testing.T, hostKey ssh.Signer) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { l.Close() })
	conf := &ssh.ServerConfig{NoClientAuth: true}
	conf.AddHostKey(hostKey)
	go func() {
		for {
			c, err := l.Accept()
			if err != nil {
				return
			}
			go func() {
				defer c.Close()
				conn, chans, reqs, err := ssh.NewServerConn(c, conf)
				if err != nil {
					return
				}
				defer conn.Close()
				go ssh.DiscardRequests(reqs)
				for ch := range chans {
					ch.Reject(ssh.Prohibited, "")
				}
			}()
		}
	}()
	return l.Addr().String()
}

func dial(addr string) error {
	conf := &ssh.ClientConfig{
		User:            "root",
		Auth:            []ssh.AuthMethod{ssh.Password("x")},
		HostKeyCallback: tofuHostKeyCallback(),
	}
	c, err := ssh.Dial("tcp", addr, conf)
	if err != nil {
		return err
	}
	return c.Close()
}

func TestTofuHostKeyCallback(t *testing.T) {
	path := filepath.Join(t.TempDir(), "known_hosts")
	SetKnownHostsPath(path)
	defer SetKnownHostsPath("")

	addr := startServer(t, newSigner(t))
	if err := dial(addr); err != nil {
		t.Fatalf("first connection: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil || !strings.Contains(string(data), "127.0.0.1") {
		t.Fatalf("host key was not recorded: %q, %v", data, err)
	}
	if err := dial(addr); err != nil {
		t.Fatalf("second connection: %v", err)
	}

	_, port, _ := net.SplitHostPort(addr)
	// a different key behind another address, with the recorded entry moved onto it
	other := startServer(t, newSigner(t))
	_, oport, _ := net.SplitHostPort(other)
	line := strings.Replace(string(data), "[127.0.0.1]:"+port, "[127.0.0.1]:"+oport, 1)
	if err := os.WriteFile(path, []byte(line), 0600); err != nil {
		t.Fatal(err)
	}
	err = dial(other)
	if err == nil || !strings.Contains(err.Error(), "has changed") {
		t.Fatalf("mismatch must fail, got %v", err)
	}

	n, err := ForgetHost(path, other)
	if err != nil || n != 1 {
		t.Fatalf("ForgetHost = %d, %v", n, err)
	}
	if err := dial(other); err != nil {
		t.Fatalf("after forget: %v", err)
	}
}

func TestForgetHost_DefaultPortAndOthers(t *testing.T) {
	path := filepath.Join(t.TempDir(), "known_hosts")
	content := "192.0.2.1 ecdsa-sha2-nistp256 AAAA\n[192.0.2.1]:2222 ecdsa-sha2-nistp256 AAAA\n"
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	n, err := ForgetHost(path, "192.0.2.1")
	if err != nil || n != 1 {
		t.Fatalf("ForgetHost = %d, %v", n, err)
	}
	data, _ := os.ReadFile(path)
	if strings.Contains(string(data), "192.0.2.1 ecdsa") || !strings.Contains(string(data), "[192.0.2.1]:2222") {
		t.Errorf("unexpected content: %q", data)
	}
}

func TestSshConnectTimeout(t *testing.T) {
	if sshConnectTimeout != 10 {
		t.Errorf("timeout = %d, want 10", sshConnectTimeout)
	}
}
