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
	"strings"
	"testing"
	"time"

	nucliozap "github.com/nuclio/zap"
	"github.com/stretchr/testify/suite"
)

type JobRunnerTestSuite struct {
	suite.Suite
	jobRunner *jobRunner
}

func (suite *JobRunnerTestSuite) SetupTest() {
	logger, err := nucliozap.NewNuclioZapTest("test")
	suite.Require().NoError(err)

	suite.jobRunner = &jobRunner{
		builderName:          "test",
		logger:               logger,
		builderConfiguration: &ContainerBuilderConfiguration{},
	}
}

func (suite *JobRunnerTestSuite) TestCreateContainerBuildBundleSuccess() {
	tempDir := suite.T().TempDir()
	contextDir := suite.T().TempDir()

	testFile := fmt.Sprintf("%s/test.txt", contextDir)
	suite.Require().NoError(os.WriteFile(testFile, []byte("test"), 0644))

	bundleFilename, assetPath, err := suite.jobRunner.createContainerBuildBundle(context.Background(), "test/image:latest", contextDir, tempDir)
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

	_, _, err := suite.jobRunner.createContainerBuildBundle(context.Background(), "test/image:latest", injectionPayload, tempDir)
	suite.Require().Error(err, "Expected tar to fail on non-existent contextDir")

	_, statErr := os.Stat(markerFile)
	suite.Require().True(os.IsNotExist(statErr),
		"Shell injection marker was created — injection succeeded via shell execution")
}

func (suite *JobRunnerTestSuite) TestCompileJobNamePrefix() {
	for _, testCase := range []struct {
		name           string
		jobPrefix      string
		image          string
		expectedPrefix string
	}{
		{
			name:           "ShortNameNoTruncation",
			jobPrefix:      "buildjob",
			image:          "my-func:latest",
			expectedPrefix: "nuclio-buildjob.my-func-latest-",
		},
		{
			name:           "SanitizesInvalidChars",
			jobPrefix:      "buildjob",
			image:          "Registry.Example.Com/My_Func:v1.2",
			expectedPrefix: "nuclio-buildjob.registry.example.com-myfunc-v1.2-",
		},
		{
			name:           "TruncatesLongNameGenerically",
			jobPrefix:      "buildjob",
			image:          strings.Repeat("x", 80) + ":latest",
			expectedPrefix: "nuclio-buildjob." + strings.Repeat("x", 41) + "-",
		},
		{
			name:           "TruncationLandsOnDot",
			jobPrefix:      "buildjob",
			image:          strings.Repeat("a", 40) + "." + strings.Repeat("b", 40) + ":latest",
			expectedPrefix: "nuclio-buildjob." + strings.Repeat("a", 40) + "-",
		},
		{
			name:           "TruncationLandsOnDash",
			jobPrefix:      "buildjob",
			image:          strings.Repeat("a", 40) + "-" + strings.Repeat("b", 40) + ":latest",
			expectedPrefix: "nuclio-buildjob." + strings.Repeat("a", 40) + "-",
		},
	} {
		suite.Run(testCase.name, func() {
			suite.jobRunner.builderConfiguration.JobPrefix = testCase.jobPrefix

			prefix := suite.jobRunner.compileJobNamePrefix(testCase.image)

			suite.Equal(testCase.expectedPrefix, prefix)
			suite.LessOrEqual(len(prefix), 58, "must leave room for k8s's 5-char GenerateName suffix")
		})
	}
}

func TestJobRunnerTestSuite(t *testing.T) {
	suite.Run(t, new(JobRunnerTestSuite))
}
