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

package runtime

import (
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/processor"

	"github.com/nuclio/logger"
	"github.com/nuclio/zap"
	"github.com/stretchr/testify/suite"
	v1 "k8s.io/api/core/v1"
)

type EnvironmentTestSuite struct {
	suite.Suite
	logger logger.Logger
}

func (suite *EnvironmentTestSuite) SetupTest() {
	var err error
	suite.logger, err = nucliozap.NewNuclioZapTest("test")
	suite.Require().NoError(err)
}

// createAbstractRuntime creates a bare abstract runtime, skipping NewAbstractRuntime's data binding
// and context creation - the environment resolution under test needs nothing but the configuration
func (suite *EnvironmentTestSuite) createAbstractRuntime(functionName string,
	env []v1.EnvVar) *AbstractRuntime {

	configuration := &Configuration{
		Configuration: &processor.Configuration{
			Config: functionconfig.Config{
				Meta: functionconfig.Meta{Name: functionName},
				Spec: functionconfig.Spec{Env: env},
			},
		},
	}

	return &AbstractRuntime{
		Logger:        suite.logger,
		configuration: configuration,
		Statistics:    &Statistics{},
	}
}

func (suite *EnvironmentTestSuite) TestGetEnvFromConfigurationIncludesSpecEnv() {
	abstractRuntime := suite.createAbstractRuntime("some-function", []v1.EnvVar{
		{Name: "MY_ENV", Value: "my-value"},
		{Name: "MY_OTHER_ENV", Value: "my-other-value"},
	})

	env := abstractRuntime.GetEnvFromConfiguration()

	// the function metadata variables come first, the spec's variables last, so that they override
	// whatever the process inherited from the pod
	suite.Require().Equal([]string{
		"NUCLIO_FUNCTION_NAME=some-function",
		"NUCLIO_FUNCTION_DESCRIPTION=",
		"NUCLIO_FUNCTION_VERSION=0",
		"NUCLIO_FUNCTION_HANDLER=",
		"MY_ENV=my-value",
		"MY_OTHER_ENV=my-other-value",
	}, env)
}

func (suite *EnvironmentTestSuite) TestGetEnvFromSpec() {
	for _, testCase := range []struct {
		name     string
		env      []v1.EnvVar
		expected []string
	}{
		{
			name:     "Empty",
			env:      nil,
			expected: nil,
		},
		{
			name:     "Plain",
			env:      []v1.EnvVar{{Name: "MY_ENV", Value: "my-value"}},
			expected: []string{"MY_ENV=my-value"},
		},
		{
			name:     "EmptyValue",
			env:      []v1.EnvVar{{Name: "MY_ENV", Value: ""}},
			expected: []string{"MY_ENV="},
		},
		{
			name: "ValueWithEqualSign",
			env: []v1.EnvVar{
				{Name: "MY_ENV", Value: "key=value=with=equals"},
			},
			expected: []string{"MY_ENV=key=value=with=equals"},
		},
		{

			// kubernetes resolves these into the pod environment; the spec holds no value for them,
			// so emitting them would shadow the resolved value with an empty string
			name: "SkipsValueFrom",
			env: []v1.EnvVar{
				{Name: "MY_ENV", Value: "my-value"},
				{
					Name: "MY_SECRET_ENV",
					ValueFrom: &v1.EnvVarSource{
						SecretKeyRef: &v1.SecretKeySelector{
							LocalObjectReference: v1.LocalObjectReference{Name: "my-secret"},
							Key:                  "my-key",
						},
					},
				},
			},
			expected: []string{"MY_ENV=my-value"},
		},
		{

			// an unresolved reference means the function secret had no matching entry - passing the
			// placeholder on would only make it look restored
			name: "SkipsUnrestoredReference",
			env: []v1.EnvVar{
				{Name: "MY_ENV", Value: "my-value"},
				{Name: "MY_MASKED_ENV", Value: functionconfig.ReferencePrefix + "/spec/env[1]/value"},
			},
			expected: []string{"MY_ENV=my-value"},
		},
	} {
		suite.Run(testCase.name, func() {
			abstractRuntime := suite.createAbstractRuntime("some-function", testCase.env)
			suite.Require().Equal(testCase.expected, abstractRuntime.GetEnvFromSpec())
		})
	}
}

// TestSpecEnvOverridesInheritedEnv asserts the end-to-end guarantee the runtimes rely on: they seed
// the child process environment with os.Environ() - which on a masked deployment holds "$ref:..."
// placeholders - and append GetEnvFromConfiguration(), whose values must win
func (suite *EnvironmentTestSuite) TestSpecEnvOverridesInheritedEnv() {
	const envName = "NUCLIO_TEST_MASKED_ENV"

	suite.T().Setenv(envName, functionconfig.ReferencePrefix+"/spec/env[0]/value")

	abstractRuntime := suite.createAbstractRuntime("some-function", []v1.EnvVar{
		{Name: envName, Value: "the-real-value"},
	})

	// mimic what the python/nodejs/java/ruby/dotnetcore runtimes do when spawning their wrapper
	command := exec.Command("env") // nolint: gosec
	command.Env = append(os.Environ(), abstractRuntime.GetEnvFromConfiguration()...)

	output, err := command.Output()
	suite.Require().NoError(err)

	var resolvedValue string
	for _, line := range strings.Split(string(output), "\n") {
		if name, value, found := strings.Cut(line, "="); found && name == envName {
			resolvedValue = value
		}
	}

	suite.Require().Equal("the-real-value", resolvedValue)
}

func TestEnvironmentTestSuite(t *testing.T) {
	suite.Run(t, new(EnvironmentTestSuite))
}
