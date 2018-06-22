/*
Copyright 2017 The Nuclio Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the Licensg.
You may obtain a copy of the License at

    http://www.apachg.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the Licensg.
*/

package build

import (
	"encoding/base64"
	"io/ioutil"
	"testing"

	"github.com/nuclio/nuclio/pkg/common"
	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/platform"
	"github.com/nuclio/nuclio/pkg/processor/build/runtime"

	"github.com/nuclio/logger"
	"github.com/nuclio/zap"
	"github.com/rs/xid"
	"github.com/stretchr/testify/suite"
)

//
// Test suite
//

type testSuite struct {
	suite.Suite
	logger  logger.Logger
	builder *Builder
	testID  string
}

// SetupSuite is called for suite setup
func (suite *testSuite) SetupSuite() {
	var err error

	suite.logger, err = nucliozap.NewNuclioZapTest("test")
	suite.Require().NoError(err)
}

// SetupTest is called before each test in the suite
func (suite *testSuite) SetupTest() {
	var err error
	suite.testID = xid.New().String()

	suite.builder, err = NewBuilder(suite.logger, nil)
	if err != nil {
		suite.Fail("Instantiating Builder failed:", err)
	}

	createFunctionOptions := &platform.CreateFunctionOptions{
		Logger:         suite.logger,
		FunctionConfig: *functionconfig.NewConfig(),
	}

	createFunctionBuildOptions := &platform.CreateFunctionBuildOptions{
		Logger:         createFunctionOptions.Logger,
		FunctionConfig: createFunctionOptions.FunctionConfig,
	}

	suite.builder.options = createFunctionBuildOptions
}

// Make sure that "Builder.getRuntimeName" properly reads the runtime name from the configuration given by the user
func (suite *testSuite) TestGetRuntimeNameFromConfig() {
	suite.builder.options.FunctionConfig.Spec.Runtime = "foo"
	runtimeName, err := suite.builder.getRuntimeName()

	if err != nil {
		suite.Fail(err.Error())
	}

	suite.Require().Equal("foo", runtimeName)
}

// Make sure that "Builder.getRuntimeName" properly reads the runtime name from the build path if not set by the user
func (suite *testSuite) TestGetPythonRuntimeNameFromBuildPath() {
	suite.builder.options.FunctionConfig.Spec.Runtime = ""
	suite.builder.options.FunctionConfig.Spec.Build.Path = "/foo.py"
	runtimeName, err := suite.builder.getRuntimeName()

	suite.Require().NoError(err)

	suite.Require().Equal("python", runtimeName)
}

// Make sure that "Builder.getRuntimeName" properly reads the runtime name from the build path if not set by the user
func (suite *testSuite) TestGetGoRuntimeNameFromBuildPath() {
	suite.builder.options.FunctionConfig.Spec.Runtime = ""
	suite.builder.options.FunctionConfig.Spec.Build.Path = "/foo.go"
	runtimeName, err := suite.builder.getRuntimeName()

	suite.Require().NoError(err)

	suite.Require().Equal("golang", runtimeName)
}

// Make sure that "Builder.getRuntimeName" returns an error if the user sends an unknown file extension without runtime
func (suite *testSuite) TestGetRuntimeNameFromBuildPathFailsOnUnknownExtension() {
	suite.builder.options.FunctionConfig.Spec.Runtime = ""
	suite.builder.options.FunctionConfig.Spec.Build.Path = "/foo.bar"
	_, err := suite.builder.getRuntimeName()

	suite.Require().Error(err, "Unsupported file extension: %s", "bar")
}

// Make sure that "Builder.getRuntimeName()" fails when the runtime is empty, and the build path is a directory
func (suite *testSuite) TestGetRuntimeNameFromBuildDirNoRuntime() {
	suite.builder.options.FunctionConfig.Spec.Runtime = ""
	suite.builder.options.FunctionConfig.Spec.Build.Path = "/user/"
	_, err := suite.builder.getRuntimeName()

	if err == nil {
		suite.Fail("Builder.getRuntimeName() should fail when given a directory for a build path and no runtime")
	}
}

func (suite *testSuite) TestWriteFunctionSourceCodeToTempFileWritesReturnsFilePath() {
	functionSourceCode := "echo foo"
	encodedFunctionSourceCode := base64.StdEncoding.EncodeToString([]byte(functionSourceCode))
	suite.builder.options.FunctionConfig.Spec.Runtime = "shell"
	suite.builder.options.FunctionConfig.Spec.Build.FunctionSourceCode = encodedFunctionSourceCode
	suite.builder.options.FunctionConfig.Spec.Build.Path = ""

	err := suite.builder.createTempDir()
	suite.Assert().NoError(err)
	defer suite.builder.cleanupTempDir()

	tempPath, err := suite.builder.writeFunctionSourceCodeToTempFile(suite.builder.options.FunctionConfig.Spec.Build.FunctionSourceCode)
	suite.Assert().NoError(err)
	suite.NotNil(tempPath)

	resultSourceCode, err := ioutil.ReadFile(tempPath)
	suite.Assert().NoError(err)

	suite.Assert().Equal(functionSourceCode, string(resultSourceCode))
}

func (suite *testSuite) TestWriteFunctionSourceCodeToTempFileFailsOnUnknownExtension() {
	suite.builder.options.FunctionConfig.Spec.Runtime = "bar"
	suite.builder.options.FunctionConfig.Spec.Build.FunctionSourceCode = base64.StdEncoding.EncodeToString([]byte("echo foo"))
	suite.builder.options.FunctionConfig.Spec.Build.Path = ""

	err := suite.builder.createTempDir()
	suite.Assert().NoError(err)
	defer suite.builder.cleanupTempDir()

	_, err = suite.builder.writeFunctionSourceCodeToTempFile(suite.builder.options.FunctionConfig.Spec.Build.FunctionSourceCode)
	suite.Assert().Error(err)
}

func (suite *testSuite) TestGetImage() {

	// user specified
	suite.builder.options.FunctionConfig.Spec.Build.Image = "userSpecified"
	suite.Require().Equal("userSpecified", suite.builder.getImage())

	// set function name and clear image name
	suite.builder.options.FunctionConfig.Meta.Name = "test"
	suite.builder.options.FunctionConfig.Spec.Build.Image = ""

	// registry has no repository - should see "nuclio/" as repository
	suite.builder.options.FunctionConfig.Spec.Build.Registry = "localhost:5000"
	suite.Require().Equal("nuclio/processor-test", suite.builder.getImage())

	// registry has a repository - should not see "nuclio/" as repository
	suite.builder.options.FunctionConfig.Spec.Build.Registry = "registry.hub.docker.com/foo"
	suite.Require().Equal("processor-test", suite.builder.getImage())

	// registry has a repository - should not see "nuclio/" as repository
	suite.builder.options.FunctionConfig.Spec.Build.Registry = "index.docker.io/foo"
	suite.Require().Equal("processor-test", suite.builder.getImage())
}

func (suite *testSuite) TestGenerateProcessorDockerfile() {

	// all elements, health check required
	suite.generateDockerfileAndVerify(true, &runtime.ProcessorDockerfileInfo{
		BaseImage: "baseImage",
		OnbuildImage: "onbuildImage",
		OnbuildArtifactPaths: map[string]string{
			"onbuildLocal1": "onbuildImage1",
			"onbuildLocal2": "onbuildImage2",
		},
		ImageArtifactPaths: map[string]string{
			"imageLocal1": "imageImage1",
			"imageLocal2": "imageImage2",
		},
		Directives: map[string][]functionconfig.Directive{
			"preCopy": {
				{Kind: "preCopyKind1", Value: "preCopyValue1"},
				{Kind: "preCopyKind2", Value: "preCopyValue2"},
			},
			"postCopy": {
				{Kind: "postCopyKind1", Value: "postCopyValue1"},
				{Kind: "postCopyKind2", Value: "postCopyValue2"},
			},
		},
	}, `# From the base image
FROM baseImage
# Run the pre-copy directives
preCopyKind1 preCopyValue1
preCopyKind2 preCopyValue2
# Copy health checker
COPY artifacts/uhttpc /usr/local/bin/uhttpc
# Readiness probe
HEALTHCHECK --interval=1s --timeout=3s CMD /usr/local/bin/uhttpc --url http://127.0.0.1:8082/ready || exit 1
# Copy required objects from the suppliers
COPY artifactDirNameInStaging/onbuildLocal1 onbuildImage1
COPY artifactDirNameInStaging/onbuildLocal2 onbuildImage2
COPY imageLocal1 imageImage1
COPY imageLocal2 imageImage2
# Run the post-copy directives
postCopyKind1 postCopyValue1
postCopyKind2 postCopyValue2
# Run processor with configuration and platform configuration
CMD [ "processor", "--config", "/etc/nuclio/config/processor/processor.yaml", "--platform-config", "/etc/nuclio/config/platform/platform.yaml" ]`)

	// all elements, health check not required
	suite.generateDockerfileAndVerify(false, &runtime.ProcessorDockerfileInfo{
		BaseImage: "baseImage",
		OnbuildImage: "onbuildImage",
		OnbuildArtifactPaths: map[string]string{
			"onbuildLocal1": "onbuildImage1",
			"onbuildLocal2": "onbuildImage2",
		},
		ImageArtifactPaths: map[string]string{
			"imageLocal1": "imageImage1",
			"imageLocal2": "imageImage2",
		},
		Directives: map[string][]functionconfig.Directive{
			"preCopy": {
				{Kind: "preCopyKind1", Value: "preCopyValue1"},
				{Kind: "preCopyKind2", Value: "preCopyValue2"},
			},
			"postCopy": {
				{Kind: "postCopyKind1", Value: "postCopyValue1"},
				{Kind: "postCopyKind2", Value: "postCopyValue2"},
			},
		},
	}, `# From the base image
FROM baseImage
# Run the pre-copy directives
preCopyKind1 preCopyValue1
preCopyKind2 preCopyValue2
# Copy required objects from the suppliers
COPY artifactDirNameInStaging/onbuildLocal1 onbuildImage1
COPY artifactDirNameInStaging/onbuildLocal2 onbuildImage2
COPY imageLocal1 imageImage1
COPY imageLocal2 imageImage2
# Run the post-copy directives
postCopyKind1 postCopyValue1
postCopyKind2 postCopyValue2
# Run processor with configuration and platform configuration
CMD [ "processor", "--config", "/etc/nuclio/config/processor/processor.yaml", "--platform-config", "/etc/nuclio/config/platform/platform.yaml" ]`)
}

func (suite *testSuite) generateDockerfileAndVerify(healthCheckRequired bool,
	dockerfileInfo *runtime.ProcessorDockerfileInfo,
	expectedDockerfile string) {

	dockerfileContents, err := suite.builder.generateSingleStageDockerfileContents("artifactDirNameInStaging",
		healthCheckRequired,
		dockerfileInfo)

	suite.Require().NoError(err)
	suite.Require().Equal(expectedDockerfile, common.RemoveEmptyLines(dockerfileContents))
}


func TestBuilderSuite(t *testing.T) {
	if testing.Short() {
		return
	}

	suite.Run(t, new(testSuite))
}
