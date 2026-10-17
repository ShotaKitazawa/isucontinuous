package shell

import (
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

const sshConnectTimeout = 10 // seconds

var (
	knownHostsMu   sync.Mutex
	knownHostsPath string
)

// DefaultKnownHostsPath is a dedicated file because ~/.ssh/known_hosts would reject recreated ISUCON servers sharing an IP.
func DefaultKnownHostsPath() string {
	return filepath.Join(os.Getenv("HOME"), ".isucontinuous_known_hosts")
}

// SetKnownHostsPath is package-level so that the usecase constructors need not carry it.
func SetKnownHostsPath(path string) {
	knownHostsMu.Lock()
	defer knownHostsMu.Unlock()
	knownHostsPath = path
}

func currentKnownHostsPath() string {
	if knownHostsPath == "" {
		return DefaultKnownHostsPath()
	}
	return knownHostsPath
}

// tofuHostKeyCallback accepts and records unknown hosts, and rejects changed keys without any bypass.
func tofuHostKeyCallback() ssh.HostKeyCallback {
	return func(hostname string, remote net.Addr, key ssh.PublicKey) error {
		knownHostsMu.Lock()
		defer knownHostsMu.Unlock()
		path := currentKnownHostsPath()
		f, err := os.OpenFile(path, os.O_RDONLY|os.O_CREATE, 0600)
		if err != nil {
			return fmt.Errorf("unable to open known_hosts: %v", err)
		}
		f.Close()
		cb, err := knownhosts.New(path)
		if err != nil {
			return fmt.Errorf("unable to read known_hosts %s: %v", path, err)
		}
		err = cb(hostname, remote, key)
		var keyErr *knownhosts.KeyError
		if err == nil || !errors.As(err, &keyErr) {
			return err
		}
		if len(keyErr.Want) > 0 {
			return fmt.Errorf(
				"host key for %s has changed (possible man-in-the-middle attack): if the server was recreated, run `isucontinuous forget %s` (known_hosts: %s)",
				hostname, hostname, path)
		}
		out, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0600)
		if err != nil {
			return fmt.Errorf("unable to record host key: %v", err)
		}
		defer out.Close()
		_, err = fmt.Fprintln(out, knownhosts.Line([]string{knownhosts.Normalize(hostname)}, key))
		return err
	}
}

// ForgetHost removes the entries of addr ("host" or "host:port") and returns how many lines were dropped.
func ForgetHost(path, addr string) (int, error) {
	if _, _, err := net.SplitHostPort(addr); err != nil {
		addr = net.JoinHostPort(strings.Trim(addr, "[]"), "22")
	}
	target := knownhosts.Normalize(addr)
	knownHostsMu.Lock()
	defer knownHostsMu.Unlock()
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	var kept []string
	removed := 0
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) > 0 && !strings.HasPrefix(fields[0], "#") && matchesHost(fields, target) {
			removed++
			continue
		}
		kept = append(kept, line)
	}
	if removed == 0 {
		return 0, nil
	}
	return removed, os.WriteFile(path, []byte(strings.Join(kept, "\n")), 0600)
}

func matchesHost(fields []string, target string) bool {
	hosts := fields[0]
	if len(fields) > 1 && strings.HasPrefix(fields[0], "@") {
		hosts = fields[1]
	}
	for _, h := range strings.Split(hosts, ",") {
		if h == target {
			return true
		}
	}
	return false
}
