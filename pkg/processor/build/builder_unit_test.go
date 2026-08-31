//go:build test_unit

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

package build

import (
	"context"
	"testing"

	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/platform"
	mockplatform "github.com/nuclio/nuclio/pkg/platform/mock"
	nucliozap "github.com/nuclio/zap"
	"github.com/stretchr/testify/require"
)

func TestResolveFunctionPathTreatsSSHRepositoryAsURL(t *testing.T) {
	logger, err := nucliozap.NewNuclioZapTest("builder")
	require.NoError(t, err)

	builder, err := NewBuilder(logger, &mockplatform.Platform{}, nil)
	require.NoError(t, err)
	builder.options = &platform.CreateFunctionBuildOptions{
		FunctionConfig: functionconfig.Config{
			Spec: functionconfig.Spec{
				Build: functionconfig.Build{
					Path:                "ssh://git@example.invalid/repository.git",
					CodeEntryType:       GitEntryType,
					CodeEntryAttributes: map[string]interface{}{},
				},
			},
		},
	}

	require.NoError(t, builder.createTempDir())
	defer builder.cleanupTempDir() // nolint: errcheck

	_, _, err = builder.resolveFunctionPath(context.Background(), builder.options.FunctionConfig.Spec.Build.Path)
	require.Error(t, err)
	require.Contains(t, err.Error(), "Failed to download function from the given URL")
	require.NotContains(t, err.Error(), "Function path doesn't exist")
}
