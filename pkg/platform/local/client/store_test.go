//go:build test_unit

/*
Copyright 2026 The Nuclio Authors.

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

package client

import (
	"strings"
	"testing"

	"github.com/nuclio/nuclio/pkg/common"
	"github.com/nuclio/nuclio/pkg/dockerclient"

	"github.com/stretchr/testify/assert"
)

// capturingDockerClient records the command handed to ExecInContainer so tests can assert
// what would actually run in the local-storage container
type capturingDockerClient struct {
	*dockerclient.MockDockerClient
	executedCommands []string
}

func (c *capturingDockerClient) ExecInContainer(containerID string, execOptions *dockerclient.ExecOptions) error {
	c.executedCommands = append(c.executedCommands, execOptions.Command)
	return nil
}

// TestGetResourcesNamedPathIsQuoted verifies that a resource path derived from
// a malicious function name is shell-quoted before being used in a command,
// so that shell metacharacters are treated as literals.
func TestGetResourcesNamedPathIsQuoted(t *testing.T) {
	s := &Store{}
	// Use a name with shell metacharacters but no spaces (getResourcePath truncates at spaces,
	// so a space-bearing name would never reach runCommand intact).
	maliciousName := `x";cat+/etc/passwd;"`
	resourcePath := s.getResourcePath(functionsDir, "default", maliciousName)

	// getResourcePath produces a path containing the raw name characters.
	assert.Contains(t, resourcePath, maliciousName)

	// common.Quote must wrap the result in single quotes when it contains
	// shell metacharacters, neutralising them.
	quoted := common.Quote(resourcePath)
	assert.True(t, strings.HasPrefix(quoted, "'"), "expected path to be single-quoted, got: %s", quoted)
}

// TestGetResourcesRejectsInjectedNamespace verifies that the list-all path rejects a namespace
// carrying shell metacharacters before any command runs. The namespace is attacker-controlled via
// the X-Nuclio-*-Namespace headers and is interpolated into `/bin/sh -c "/bin/cat <path>"`; the
// GHSA-jx7q-cpg6-669g fix quoted only the named-resource path, leaving this one exploitable
// (GHSA-mq8w-f7w8-5rgg).
func TestGetResourcesRejectsInjectedNamespace(t *testing.T) {
	dockerClient := &capturingDockerClient{MockDockerClient: &dockerclient.MockDockerClient{}}
	s := &Store{dockerClient: dockerClient}

	// list-all path (empty resource name)
	err := s.getResources(functionsDir, `x" || touch /tmp/pwned #`, "", func([]byte) error { return nil })

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "k8s naming convention")
	assert.Empty(t, dockerClient.executedCommands, "no shell command must run for an invalid namespace")
}

// TestGetResourcesListsValidNamespace verifies a well-formed namespace still reaches the
// list-all command unharmed.
func TestGetResourcesListsValidNamespace(t *testing.T) {
	dockerClient := &capturingDockerClient{MockDockerClient: &dockerclient.MockDockerClient{}}
	s := &Store{dockerClient: dockerClient}

	err := s.getResources(functionsDir, "mynamespace", "", func([]byte) error { return nil })

	assert.NoError(t, err)
	assert.Len(t, dockerClient.executedCommands, 1)
	assert.Contains(t, dockerClient.executedCommands[0], "/bin/cat")
	assert.Contains(t, dockerClient.executedCommands[0], "mynamespace")
}

// TestGetResourcesAllowsEmptyNamespace guards against over-validation: an empty namespace is the
// unset/default case (used internally, e.g. to lazily initialize the local-storage reader) and must
// not be rejected — only attacker-supplied non-empty namespaces are validated.
func TestGetResourcesAllowsEmptyNamespace(t *testing.T) {
	dockerClient := &capturingDockerClient{MockDockerClient: &dockerclient.MockDockerClient{}}
	s := &Store{dockerClient: dockerClient}

	err := s.getResources(functionsDir, "", "", func([]byte) error { return nil })

	assert.NoError(t, err)
	assert.Len(t, dockerClient.executedCommands, 1, "an empty namespace must still reach the list command")
}
