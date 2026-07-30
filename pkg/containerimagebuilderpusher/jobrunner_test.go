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

	"github.com/nuclio/nuclio/pkg/containerimagebuilderpusher/registryhelpers"
	"github.com/nuclio/nuclio/pkg/platform/kube/clients/kube"

	nucliozap "github.com/nuclio/zap"
	"github.com/stretchr/testify/suite"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sruntime "k8s.io/apimachinery/pkg/runtime"
	k8sfake "k8s.io/client-go/kubernetes/fake"
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

// TestEnsureMergeScriptConfigMap covers the server-side-apply upsert from each initial state.
func (suite *JobRunnerTestSuite) TestEnsureMergeScriptConfigMap() {
	for _, testCase := range []struct {
		name            string
		existingScript  string
		configMapExists bool
	}{
		{name: "CreatesWhenAbsent"},
		{name: "UpdatesWhenStale", configMapExists: true, existingScript: "stale contents"},
		{name: "IdempotentWhenUpToDate", configMapExists: true, existingScript: registryhelpers.MergeScriptContents()},
	} {
		suite.Run(testCase.name, func() {
			var existingObjects []k8sruntime.Object
			if testCase.configMapExists {
				existingObjects = append(existingObjects, &v1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{Name: registryhelpers.MergeScriptConfigMapName, Namespace: "default"},
					Data:       map[string]string{"merge_authfile.py": testCase.existingScript},
				})
			}
			suite.jobRunner.kubeClientSet = kube.NewClientWithRetryFromClient(k8sfake.NewClientset(existingObjects...))

			suite.Require().NoError(suite.jobRunner.ensureMergeScriptConfigMap(context.Background(), "default"))

			configMap, err := suite.jobRunner.kubeClientSet.GetConfigMap(context.Background(),
				"default",
				registryhelpers.MergeScriptConfigMapName)
			suite.Require().NoError(err)
			suite.Equal(registryhelpers.MergeScriptContents(), configMap.Data["merge_authfile.py"])
		})
	}
}

func TestJobRunnerTestSuite(t *testing.T) {
	suite.Run(t, new(JobRunnerTestSuite))
}
