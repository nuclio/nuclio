/*
Copyright 2023 The Nuclio Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Package gitssh provides a disposable real SSH Git server for local integration tests.
package gitssh

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

const passphrase = "test-passphrase"

// Fixture describes a temporary bare Git repository served by a real sshd process.
type Fixture struct {
	RepositoryURL        string
	PrivateKey           string
	PassphrasePrivateKey string
	Passphrase           string
	KnownHosts           string
	WrongPrivateKey      string
	WrongKnownHosts      string
}

// New starts a real sshd process and returns credentials for cloning its bare Git repository.
// repositoryHost is the hostname that clients use in both RepositoryURL and KnownHosts.
func New(t *testing.T, repositoryHost string) *Fixture {
	t.Helper()

	for _, binary := range []string{"sshd", "ssh-keygen", "git"} {
		if _, err := exec.LookPath(binary); err != nil {
			t.Skipf("%s is required for the real SSH Git-server test", binary)
		}
	}

	currentUser, err := user.Current()
	require.NoError(t, err)

	root := t.TempDir()
	repoPath := filepath.Join(root, "repository.git")
	seedPath := filepath.Join(root, "seed")
	createRepository(t, repoPath, seedPath)

	privateKeyPath := filepath.Join(root, "client-key")
	generateKey(t, privateKeyPath, "")
	passphrasePrivateKeyPath := filepath.Join(root, "encrypted-client-key")
	generateKey(t, passphrasePrivateKeyPath, passphrase)

	wrongPrivateKeyPath := filepath.Join(root, "wrong-client-key")
	generateKey(t, wrongPrivateKeyPath, "")

	authorizedKeysPath := filepath.Join(root, "authorized_keys")
	authorizedKeys := strings.TrimSpace(readFile(t, privateKeyPath+".pub")) + "\n" +
		strings.TrimSpace(readFile(t, passphrasePrivateKeyPath+".pub")) + "\n"
	require.NoError(t, os.WriteFile(authorizedKeysPath, []byte(authorizedKeys), 0600))

	hostKeyPath := filepath.Join(root, "host-key")
	generateKey(t, hostKeyPath, "")
	wrongHostKeyPath := filepath.Join(root, "wrong-host-key")
	generateKey(t, wrongHostKeyPath, "")

	port := findFreePort(t)
	configPath := filepath.Join(root, "sshd_config")
	config := fmt.Sprintf("Port %d\nListenAddress 0.0.0.0\nHostKey %s\nAuthorizedKeysFile %s\nPidFile %s\n"+
		"PasswordAuthentication no\nKbdInteractiveAuthentication no\nChallengeResponseAuthentication no\n"+
		"UsePAM no\nPermitRootLogin yes\nStrictModes no\nAllowUsers %s\nLogLevel ERROR\n",
		port, hostKeyPath, authorizedKeysPath, filepath.Join(root, "sshd.pid"), currentUser.Username)
	require.NoError(t, os.WriteFile(configPath, []byte(config), 0600))

	sshdPath, err := exec.LookPath("sshd")
	require.NoError(t, err)
	logFile, err := os.Create(filepath.Join(root, "sshd.log"))
	require.NoError(t, err)

	sshd := exec.Command(sshdPath, "-D", "-e", "-f", configPath)
	sshd.Stdout = logFile
	sshd.Stderr = logFile
	require.NoError(t, sshd.Start())
	t.Cleanup(func() {
		_ = sshd.Process.Kill()
		_ = sshd.Wait()
		_ = logFile.Close()
	})

	waitForPort(t, port)
	hostKeyFields := strings.Fields(readFile(t, hostKeyPath+".pub"))
	require.Len(t, hostKeyFields, 3)
	wrongHostKeyFields := strings.Fields(readFile(t, wrongHostKeyPath+".pub"))
	require.Len(t, wrongHostKeyFields, 3)

	return &Fixture{
		RepositoryURL:        fmt.Sprintf("ssh://%s@%s:%d%s", currentUser.Username, repositoryHost, port, repoPath),
		PrivateKey:           readFile(t, privateKeyPath),
		PassphrasePrivateKey: readFile(t, passphrasePrivateKeyPath),
		Passphrase:           passphrase,
		KnownHosts:           fmt.Sprintf("[%s]:%d %s %s\n", repositoryHost, port, hostKeyFields[0], hostKeyFields[1]),
		WrongPrivateKey:      readFile(t, wrongPrivateKeyPath),
		WrongKnownHosts:      fmt.Sprintf("[%s]:%d %s %s\n", repositoryHost, port, wrongHostKeyFields[0], wrongHostKeyFields[1]),
	}
}

func createRepository(t *testing.T, repoPath, seedPath string) {
	t.Helper()
	runCommand(t, "git", "init", "--bare", repoPath)
	runCommand(t, "git", "init", seedPath)
	runCommand(t, "git", "-C", seedPath, "config", "user.email", "git-ssh-test@example.invalid")
	runCommand(t, "git", "-C", seedPath, "config", "user.name", "Git SSH Test")
	require.NoError(t, os.WriteFile(filepath.Join(seedPath, "function.yaml"), []byte("spec:\n  runtime: shell\n  handler: handler.sh:main\n"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(seedPath, "handler.sh"),
		[]byte("#!/bin/sh\nmain() {\n  cat\n}\n"), 0755))
	runCommand(t, "git", "-C", seedPath, "add", ".")
	runCommand(t, "git", "-C", seedPath, "commit", "-m", "SSH integration fixture")
	runCommand(t, "git", "-C", seedPath, "branch", "-M", "ssh-test")
	runCommand(t, "git", "-C", seedPath, "remote", "add", "origin", repoPath)
	runCommand(t, "git", "-C", seedPath, "push", "origin", "ssh-test")
}

func generateKey(t *testing.T, path, keyPassphrase string) {
	t.Helper()
	runCommand(t, "ssh-keygen", "-q", "-t", "ed25519", "-N", keyPassphrase, "-f", path)
}

func findFreePort(t *testing.T) int {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	port := listener.Addr().(*net.TCPAddr).Port
	require.NoError(t, listener.Close())
	return port
}

func waitForPort(t *testing.T, port int) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		connection, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), 250*time.Millisecond)
		if err == nil {
			_ = connection.Close()
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for sshd on port %d", port)
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	contents, err := os.ReadFile(path)
	require.NoError(t, err)
	return string(contents)
}

func runCommand(t *testing.T, name string, args ...string) []byte {
	t.Helper()
	output, err := exec.Command(name, args...).CombinedOutput()
	require.NoError(t, err, "%s %s\n%s", name, strings.Join(args, " "), output)
	return output
}
