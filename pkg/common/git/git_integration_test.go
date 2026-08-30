//go:build test_integration && test_local

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

package git

import (
	"path/filepath"
	"testing"

	"github.com/nuclio/nuclio/test/e2e/gitssh"
	nucliozap "github.com/nuclio/zap"
	"github.com/stretchr/testify/require"
)

// TestCloneFromRealSSHGitServer exercises the complete go-git SSH clone against a real sshd
// serving a bare Git repository through git-upload-pack. It is intentionally opt-in because the
// test needs the host's sshd, ssh-keygen, and git binaries.
func TestCloneFromRealSSHGitServer(t *testing.T) {
	fixture := gitssh.New(t, "127.0.0.1")

	logger, err := nucliozap.NewNuclioZapTest("git-ssh-integration")
	require.NoError(t, err)
	client, err := NewClient(logger)
	require.NoError(t, err)

	t.Run("unencrypted key with strict host verification", func(t *testing.T) {
		cloneTestRepository(t, client, fixture.RepositoryURL, filepath.Join(t.TempDir(), "clone-plain"),
			fixture.PrivateKey, "", fixture.KnownHosts)
	})

	t.Run("passphrase-protected key", func(t *testing.T) {
		cloneTestRepository(t, client, fixture.RepositoryURL, filepath.Join(t.TempDir(), "clone-encrypted"),
			fixture.PassphrasePrivateKey, fixture.Passphrase, fixture.KnownHosts)
	})

	t.Run("wrong private key fails", func(t *testing.T) {
		err := client.Clone(filepath.Join(t.TempDir(), "clone-wrong-key"), fixture.RepositoryURL, &Attributes{
			Branch:        "ssh-test",
			SSHPrivateKey: fixture.WrongPrivateKey,
			SSHKnownHosts: fixture.KnownHosts,
		})
		require.Error(t, err)
	})

	t.Run("wrong known host fails", func(t *testing.T) {
		err := client.Clone(filepath.Join(t.TempDir(), "clone-wrong-host-key"), fixture.RepositoryURL, &Attributes{
			Branch:        "ssh-test",
			SSHPrivateKey: fixture.PrivateKey,
			SSHKnownHosts: fixture.WrongKnownHosts,
		})
		require.Error(t, err)
	})
}

func cloneTestRepository(t *testing.T, client Client, repositoryURL, outputPath, privateKey, passphrase, knownHosts string) {
	t.Helper()
	err := client.Clone(outputPath, repositoryURL, &Attributes{
		Branch:        "ssh-test",
		SSHPrivateKey: privateKey,
		SSHPassphrase: passphrase,
		SSHKnownHosts: knownHosts,
	})
	require.NoError(t, err)
	require.FileExists(t, filepath.Join(outputPath, "function.yaml"))
	require.FileExists(t, filepath.Join(outputPath, "handler.sh"))
}
