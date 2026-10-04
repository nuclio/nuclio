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
	"testing"

	"github.com/nuclio/nuclio/pkg/dockerclient"

	"github.com/nuclio/errors"
	"github.com/nuclio/logger"
	"github.com/stretchr/testify/require"
)

type namespaceDockerClient struct {
	*dockerclient.MockDockerClient
	output string
	err    error
}

func (c *namespaceDockerClient) ExecInContainer(_ string, options *dockerclient.ExecOptions) error {
	*options.Stdout = c.output
	return c.err
}

func TestGetNamespaces(t *testing.T) {
	for _, tc := range []struct {
		name, output string
		want         []string
	}{
		{"empty", "", []string{"nuclio"}},
		{"resources", "/etc/nuclio/store/projects/embedding/project.json\n/etc/nuclio/store/functions/embedding/function.json\n/etc/nuclio/store/function-events/tools/event.json\n", []string{"embedding", "nuclio", "tools"}},
		{"ignore unrelated", "/etc/nuclio/store/other/ignored/file.json\n/etc/nuclio/store/projects/INVALID/file.json\n", []string{"nuclio"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := &Store{dockerClient: &namespaceDockerClient{output: tc.output}}
			got, err := s.GetNamespaces()

			require.NoError(t, err)
			require.Equal(t, tc.want, got)
		})
	}
}

type namespaceLogger struct {
	logger.Logger
	warnings [][]interface{}
}

func (l *namespaceLogger) WarnWith(_ interface{}, fields ...interface{}) {
	l.warnings = append(l.warnings, fields)
}

func TestGetNamespacesRetainsDefaultOnStorageFailure(t *testing.T) {
	failure := errors.New("storage unavailable")
	log := &namespaceLogger{}
	s := &Store{dockerClient: &namespaceDockerClient{err: failure}, logger: log}
	got, err := s.GetNamespaces()

	require.NoError(t, err)
	require.Equal(t, []string{"nuclio"}, got)
	require.Len(t, log.warnings, 1)
	require.Equal(t, "err", log.warnings[0][0])
	loggedError, ok := log.warnings[0][1].(error)
	require.True(t, ok)
	require.Equal(t, failure, errors.RootCause(loggedError))
}
