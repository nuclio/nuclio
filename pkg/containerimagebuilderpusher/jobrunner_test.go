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

package containerimagebuilderpusher

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	nucliozap "github.com/nuclio/zap"
	"github.com/stretchr/testify/suite"
)

type JobRunnerTestSuite struct {
	suite.Suite
}

func (suite *JobRunnerTestSuite) TestCreateContainerBuildBundleSuccess() {
	tempDir := suite.T().TempDir()
	contextDir := suite.T().TempDir()

	testFile := fmt.Sprintf("%s/test.txt", contextDir)
	suite.Require().NoError(os.WriteFile(testFile, []byte("test"), 0644))

	logger, err := nucliozap.NewNuclioZapTest("test")
	suite.Require().NoError(err)

	r := &jobRunner{
		builderName:          "test",
		logger:               logger,
		builderConfiguration: &ContainerBuilderConfiguration{},
	}

	bundleFilename, assetPath, err := r.createContainerBuildBundle(context.Background(), "test/image:latest", contextDir, tempDir)
	suite.Require().NoError(err)
	suite.Require().NotEmpty(bundleFilename)
	suite.Require().FileExists(assetPath)
	defer os.Remove(assetPath) // nolint: errcheck
}

func (suite *JobRunnerTestSuite) TestCreateContainerBuildBundleSafeFromShellInjection() {
	tempDir := suite.T().TempDir()

	// contextDir with a semicolon-injected shell command
	markerFile := fmt.Sprintf("%s/kaniko-injection-marker-%d", os.TempDir(), time.Now().UnixNano())
	injectionPayload := fmt.Sprintf("/nonexistent-dir; touch %s", markerFile)

	logger, err := nucliozap.NewNuclioZapTest("test")
	suite.Require().NoError(err)

	r := &jobRunner{
		builderName:          "test",
		logger:               logger,
		builderConfiguration: &ContainerBuilderConfiguration{},
	}

	_, _, err = r.createContainerBuildBundle(context.Background(), "test/image:latest", injectionPayload, tempDir)
	suite.Require().Error(err, "Expected tar to fail on non-existent contextDir")

	_, statErr := os.Stat(markerFile)
	suite.Require().True(os.IsNotExist(statErr),
		"Shell injection marker was created — injection succeeded via shell execution")
}

func (suite *JobRunnerTestSuite) TestResolvePodLabelsCopiesAndIsolatesFromConfig() {
	configLabels := map[string]string{"azure.workload.identity/use": "true"}

	resolved := resolvePodLabels(configLabels)
	suite.Equal("true", resolved["azure.workload.identity/use"])

	// Mutating the returned map must not leak back into the shared config map.
	resolved["mutated"] = "yes"
	_, leaked := configLabels["mutated"]
	suite.False(leaked, "resolvePodLabels must return a copy; mutation leaked into the source map")
}

func (suite *JobRunnerTestSuite) TestResolvePodLabelsReturnsNilWhenNoLabelsConfigured() {
	suite.Nil(resolvePodLabels(nil))
}

func TestJobRunnerTestSuite(t *testing.T) {
	suite.Run(t, new(JobRunnerTestSuite))
}
