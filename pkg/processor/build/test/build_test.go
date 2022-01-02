//go:build test_integration && test_local

/*
Copyright 2017 The Nuclio Authors.

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

package buildsuite

import (
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"strings"
	"testing"

	"github.com/nuclio/nuclio/pkg/common"
	"github.com/nuclio/nuclio/pkg/containerimagebuilderpusher"
	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/platform"
	"github.com/nuclio/nuclio/pkg/platform/abstract"
	"github.com/nuclio/nuclio/pkg/platform/kube"
	"github.com/nuclio/nuclio/pkg/platform/local"
	"github.com/nuclio/nuclio/pkg/platformconfig"
	"github.com/nuclio/nuclio/pkg/processor/build"
	"github.com/nuclio/nuclio/pkg/processor/build/runtime"
	"github.com/nuclio/nuclio/pkg/processor/trigger/http/test/suite"
	"github.com/nuclio/nuclio/test/httpsrv"

	"github.com/nuclio/errors"
	"github.com/rs/xid"
	"github.com/stretchr/testify/suite"
	"k8s.io/api/core/v1"
)

type testSuite struct {
	httpsuite.TestSuite
}

func (suite *testSuite) SetupSuite() {
	suite.TestSuite.SetupSuite()
}

func (suite *testSuite) TestBuildFuncFromSourceWithInlineConfig() {
	functionSourceCode := `
# @nuclio.configure
#
# function.yaml:
#   spec:
#     env:
#     - name: MESSAGE
#       value: foo

echo $MESSAGE`
	suite.createShellFunctionFromSourceCode("test-inline-config", functionSourceCode, &httpsuite.Request{
		RequestMethod:        "POST",
		RequestBody:          "",
		ExpectedResponseBody: "foo\n",
	})
}

func (suite *testSuite) TestBuildFuncFromSourceWithWindowsCarriage() {
	functionSourceCode := `#!/bin/sh
echo 'test'`

	functionSourceCode = strings.ReplaceAll(functionSourceCode, "\n", "\r\n")
	suite.createShellFunctionFromSourceCode("test-windows-carriage", functionSourceCode, &httpsuite.Request{
		RequestMethod:        "POST",
		RequestBody:          "",
		ExpectedResponseBody: "test\n",
	})
}

func (suite *testSuite) TestBuildFunctionFromSourceCodeMaintainsSource() {
	createFunctionOptions := &platform.CreateFunctionOptions{
		Logger: suite.Logger,
	}

	functionSourceCode := base64.StdEncoding.EncodeToString([]byte(`def handler(context, event):
	pass
`))

	// simulate the case where the function path _and_ source code is provided. function source code
	// should remain untouched
	tempFile, err := ioutil.TempFile(os.TempDir(), "prefix")
	suite.Require().NoError(err)
	defer os.Remove(tempFile.Name()) // nolint: errcheck

	// we *don't* want the contents of the temp file to appear in the function source code, because
	// the function source code is already populated
	_, err = tempFile.WriteString("Contents of temp file")
	suite.Require().NoError(err)

	createFunctionOptions.FunctionConfig.Meta.Name = "funcsource-test"
	createFunctionOptions.FunctionConfig.Meta.Namespace = "default"
	createFunctionOptions.FunctionConfig.Spec.Handler = "main:handler"
	createFunctionOptions.FunctionConfig.Spec.Runtime = "python:3.6"
	createFunctionOptions.FunctionConfig.Spec.Build.Path = tempFile.Name()
	createFunctionOptions.FunctionConfig.Spec.Build.FunctionSourceCode = functionSourceCode

	suite.DeployFunction(createFunctionOptions, func(deployResult *platform.CreateFunctionResult) bool {

		// get the function
		functions, err := suite.Platform.GetFunctions(suite.TestSuite.Ctx, &platform.GetFunctionsOptions{
			Name:      createFunctionOptions.FunctionConfig.Meta.Name,
			Namespace: createFunctionOptions.FunctionConfig.Meta.Namespace,
		})

		suite.Require().NoError(err)
		suite.Require().Len(functions, 1)

		suite.Require().Equal(functionSourceCode,
			functions[0].GetConfig().Spec.Build.FunctionSourceCode)

		return true
	})
}

func (suite *testSuite) TestBuildFunctionFromSourceCodeDeployOnceNeverBuild() {
	createFunctionOptions := &platform.CreateFunctionOptions{
		Logger: suite.Logger,
	}

	functionSourceCode := base64.StdEncoding.EncodeToString([]byte(`def handler(context, event):
	pass
`))

	createFunctionOptions.FunctionConfig.Meta.Name = "neverbuild-test"
	createFunctionOptions.FunctionConfig.Meta.Namespace = "default"
	createFunctionOptions.FunctionConfig.Spec.Handler = "main:handler"
	createFunctionOptions.FunctionConfig.Spec.Runtime = "python:3.6"
	createFunctionOptions.FunctionConfig.Spec.Build.FunctionSourceCode = functionSourceCode

	// expect failure
	createFunctionOptions.FunctionConfig.Spec.Build.Mode = functionconfig.NeverBuild

	suite.DeployFunctionExpectError(createFunctionOptions, func(deployResult *platform.CreateFunctionResult) bool { // nolint: errcheck

		// get the function
		functions, err := suite.Platform.GetFunctions(suite.TestSuite.Ctx, &platform.GetFunctionsOptions{
			Name:      createFunctionOptions.FunctionConfig.Meta.Name,
			Namespace: createFunctionOptions.FunctionConfig.Meta.Namespace,
		})

		suite.Require().NoError(err)

		// verify function was not saved
		suite.Require().Empty(functions)

		// expect no deploy result
		suite.Require().Nil(deployResult)

		return true
	})
}

func (suite *testSuite) TestBuildFunctionFromSourceCodeNeverBuildRedeploy() {
	var resultFunctionConfigSpec functionconfig.Spec
	var lastBuildTimestamp int64

	createFunctionOptions := &platform.CreateFunctionOptions{
		Logger: suite.Logger,
	}

	functionSourceCode := base64.StdEncoding.EncodeToString([]byte(`def handler(context, event):
    pass
`))

	createFunctionOptions.FunctionConfig.Meta.Name = "neverbuild-redeploy-func"
	createFunctionOptions.FunctionConfig.Meta.Namespace = "default"
	createFunctionOptions.FunctionConfig.Spec.Handler = "main:handler"
	createFunctionOptions.FunctionConfig.Spec.Runtime = "python:3.6"
	createFunctionOptions.FunctionConfig.Spec.Build.FunctionSourceCode = functionSourceCode

	afterFirstDeploy := func(deployResult *platform.CreateFunctionResult) bool {

		// get the function
		functions, err := suite.Platform.GetFunctions(suite.TestSuite.Ctx, &platform.GetFunctionsOptions{
			Name:      createFunctionOptions.FunctionConfig.Meta.Name,
			Namespace: createFunctionOptions.FunctionConfig.Meta.Namespace,
		})

		suite.Require().NoError(err)
		suite.NotEqual(0, resultFunctionConfigSpec.Build.Timestamp)

		resultFunctionConfigSpec = functions[0].GetConfig().Spec

		// next deploy don't build
		createFunctionOptions.FunctionConfig.Spec.Build.Mode = functionconfig.NeverBuild
		lastBuildTimestamp = resultFunctionConfigSpec.Build.Timestamp

		return true
	}

	afterSecondDeploy := func(deployResult *platform.CreateFunctionResult) bool {

		// get the function
		functions, err := suite.Platform.GetFunctions(suite.TestSuite.Ctx, &platform.GetFunctionsOptions{
			Name:      createFunctionOptions.FunctionConfig.Meta.Name,
			Namespace: createFunctionOptions.FunctionConfig.Meta.Namespace,
		})

		suite.Require().NoError(err)
		resultFunctionConfigSpec = functions[0].GetConfig().Spec

		suite.NotEqual(0, resultFunctionConfigSpec.Build.Timestamp)
		suite.Equal(lastBuildTimestamp, resultFunctionConfigSpec.Build.Timestamp)

		// verify build mode is cleared
		suite.Require().Equal(functionconfig.BuildMode(""), functions[0].GetConfig().Spec.Build.Mode)

		return true
	}

	suite.DeployFunctionAndRedeploy(createFunctionOptions, afterFirstDeploy, afterSecondDeploy)

}

func (suite *testSuite) TestBuildFunctionFromFileExpectSourceCodePopulated() {
	createFunctionOptions := suite.GetDeployOptions("reverser",
		path.Join(suite.GetNuclioSourceDir(), "test", "_functions", "common", "reverser", "python", "reverser.py"))

	createFunctionOptions.FunctionConfig.Spec.Runtime = "python"
	createFunctionOptions.FunctionConfig.Spec.Handler = "reverser:handler"

	suite.DeployFunction(createFunctionOptions, func(deployResult *platform.CreateFunctionResult) bool {

		// get the function
		functions, err := suite.Platform.GetFunctions(suite.TestSuite.Ctx, &platform.GetFunctionsOptions{
			Name:      createFunctionOptions.FunctionConfig.Meta.Name,
			Namespace: createFunctionOptions.FunctionConfig.Meta.Namespace,
		})

		suite.Require().NoError(err)
		suite.Require().Len(functions, 1)

		// read function source code
		functionSourceCode, err := ioutil.ReadFile(createFunctionOptions.FunctionConfig.Spec.Build.Path)
		suite.Require().NoError(err)

		suite.Require().Equal(base64.StdEncoding.EncodeToString(functionSourceCode),
			functions[0].GetConfig().Spec.Build.FunctionSourceCode)

		return true
	})
}

func (suite *testSuite) TestBuildInvalidFunctionPath() {
	var err error

	createFunctionOptions := suite.GetDeployOptions("invalid", "invalidpath")

	_, err = suite.Platform.CreateFunctionBuild(&platform.CreateFunctionBuildOptions{
		Logger:         createFunctionOptions.Logger,
		FunctionConfig: createFunctionOptions.FunctionConfig,
		PlatformName:   suite.Platform.GetName(),
	})

	suite.Require().Contains(errors.Cause(err).Error(), "invalidpath")
}

func (suite *testSuite) TestBuildJessiePassesNonInteractiveFlag() {
	createFunctionOptions := suite.GetDeployOptions("jessie-non-interactive",
		path.Join(suite.GetNuclioSourceDir(), "test", "_functions", "common", "empty", "python"))

	createFunctionOptions.FunctionConfig.Spec.Runtime = "python"
	createFunctionOptions.FunctionConfig.Spec.Handler = "empty:handler"
	createFunctionOptions.FunctionConfig.Spec.Build.BaseImage = "python:3.6-jessie"

	createFunctionOptions.FunctionConfig.Spec.Build.Commands = append(createFunctionOptions.FunctionConfig.Spec.Build.Commands, "apt-get -qq update")
	createFunctionOptions.FunctionConfig.Spec.Build.Commands = append(createFunctionOptions.FunctionConfig.Spec.Build.Commands, "apt-get -qq install curl")

	statusOk := http.StatusOK
	suite.DeployFunctionAndRequest(createFunctionOptions,
		&httpsuite.Request{
			ExpectedResponseStatusCode: &statusOk,
		})
}

func (suite *testSuite) TestDockerCacheUtilized() {
	ipAddress := os.Getenv("NUCLIO_TEST_IP_ADDRESS")
	if ipAddress == "" {
		suite.T().Skip()
	}

	firstFilePath := "/first-file.txt"
	secondFilePath := "/second-file.txt"
	firstFilePattern := "/first-file"
	secondFilePattern := "/second-file"
	serverAddress := fmt.Sprintf("%s:7777", ipAddress)

	//
	// Preparation
	//

	generateSourceCode := func() (string, string) {
		randomString := xid.New().String()

		// create a function that returns the contents of both files + something random so that subsequent
		// runs of this test doesn't hit docker cache (if that's possible)
		sourceCode := fmt.Sprintf(`def handler(context, event):
	return open('%s', 'r').read() + ':' + open('%s', 'r').read() + ':%s'
`, firstFilePath, secondFilePath, randomString)

		return base64.StdEncoding.EncodeToString([]byte(sourceCode)), randomString

	}

	functionConfig := *functionconfig.NewConfig()
	functionConfig.Spec.ReadinessTimeoutSeconds = 5

	createFunctionOptions := &platform.CreateFunctionOptions{
		Logger:         suite.Logger,
		FunctionConfig: functionConfig,
	}

	createFunctionOptions.FunctionConfig.Meta.Name = "cache-test"
	createFunctionOptions.FunctionConfig.Spec.Runtime = "python:3.6"
	createFunctionOptions.FunctionConfig.Spec.Handler = "cachetest:handler"
	createFunctionOptions.FunctionConfig.Spec.Build.TempDir = suite.CreateTempDir()
	createFunctionOptions.FunctionConfig.Spec.Build.Commands = []string{
		// install curl
		"apk --update --no-cache add curl",

		// fetch first file
		fmt.Sprintf("curl -L %s%s --output %s", serverAddress, firstFilePattern, firstFilePath),

		// indicate that commands from here on out should execute _after_ the copy of artifacts
		"@nuclio.postCopy",

		// fetch second file
		fmt.Sprintf("curl -L %s%s --output %s", serverAddress, secondFilePattern, secondFilePath),
	}

	//
	// First build
	//

	firstFileFirstBuildContents := "firstFileFirstBuild"
	secondFileFirstBuildContents := "secondFileFirstBuild"

	// create an HTTP server that serves the contents
	httpServer, err := httpsrv.NewServer(serverAddress, nil, []httpsrv.ServedObject{
		{
			Contents: firstFileFirstBuildContents,
			Pattern:  firstFilePattern,
		},
		{
			Contents: secondFileFirstBuildContents,
			Pattern:  secondFilePattern,
		},
	})

	suite.Require().NoError(err)

	// generate (unique) source code
	sourceCode, randomString := generateSourceCode()
	createFunctionOptions.FunctionConfig.Spec.Build.FunctionSourceCode = sourceCode

	// do the build. expect to get the first/second file contents of the first build
	suite.DeployFunctionAndRequest(createFunctionOptions,
		&httpsuite.Request{
			RequestMethod: "GET",
			ExpectedResponseBody: fmt.Sprintf("%s:%s:%s",
				firstFileFirstBuildContents,
				secondFileFirstBuildContents,
				randomString),
		})

	// stop serving
	httpServer.Stop() // nolint: errcheck

	//
	// Second build: Don't change source code. Expect everything to come from the cache
	//

	firstFileSecondBuildContents := "firstFileSecondBuild"
	secondFileSecondBuildContents := "secondFileSecondBuild"

	// create an HTTP server that serves the contents
	httpServer, err = httpsrv.NewServer(serverAddress, nil, []httpsrv.ServedObject{
		{
			Contents: firstFileSecondBuildContents,
			Pattern:  firstFilePattern,
		},
		{
			Contents: secondFileSecondBuildContents,
			Pattern:  secondFilePattern,
		},
	})

	suite.Require().NoError(err)

	// do the build. expect to get the first/second file contents of the first build
	suite.DeployFunctionAndRequest(createFunctionOptions,
		&httpsuite.Request{
			RequestMethod: "GET",
			ExpectedResponseBody: fmt.Sprintf("%s:%s:%s",
				firstFileFirstBuildContents,
				secondFileFirstBuildContents,
				randomString),
		})

	// stop serving
	httpServer.Stop() // nolint: errcheck

	//
	// Third build: Change the source code. Expect only the second file contents to change, because the
	//              first curl should hit the cache (and not execute) whereas the second curl should
	//              not hit the cache because it should be executed after the source code copy which
	//              invalidates the cache. To illustrate the underlying Dockerfile should be:
	//
	// RUN first curl <-- should come from cache (first execution)
	// COPY source to somewhere <-- source code changed, will invalidate the cache
	// RUN second curl <-- should NOT come from the cache because the previous COPY invalidated the cache
	//

	firstFileThirdBuildContents := "firstFileThirdBuild"
	secondFileThirdBuildContents := "secondFileThirdBuild"

	// create an HTTP server that serves the contents
	httpServer, err = httpsrv.NewServer(serverAddress, nil, []httpsrv.ServedObject{
		{
			Contents: firstFileThirdBuildContents,
			Pattern:  firstFilePattern,
		},
		{
			Contents: secondFileThirdBuildContents,
			Pattern:  secondFilePattern,
		},
	})

	suite.Require().NoError(err)

	// generate (unique) source code again. This will cause cache invalidation at some point in the layers
	sourceCode, randomString = generateSourceCode()
	createFunctionOptions.FunctionConfig.Spec.Build.FunctionSourceCode = sourceCode

	// do the build. expect to get the first/second file contents of the first build
	suite.DeployFunctionAndRequest(createFunctionOptions,
		&httpsuite.Request{
			RequestMethod: "GET",
			ExpectedResponseBody: fmt.Sprintf("%s:%s:%s",
				firstFileFirstBuildContents,
				secondFileThirdBuildContents,
				randomString),
		})

	// stop serving
	httpServer.Stop() // nolint: errcheck
}

func (suite *testSuite) TestBuildFuncFromImageAndRedeploy() {

	// generate random responses
	functionAResponse := fmt.Sprintf("FunctionA-%s", xid.New().String())
	functionBResponse := fmt.Sprintf("FunctionB-%s", xid.New().String())

	// create two functions
	createAFunctionResult := suite.createShellFunctionWithResponse(functionAResponse)
	createBFunctionResult := suite.createShellFunctionWithResponse(functionBResponse)

	suite.Assert().NotEmpty(createAFunctionResult.Image)
	suite.Assert().NotEmpty(createBFunctionResult.Image)

	// codeEntryType -> image
	createFunctionFromImageOptions := &platform.CreateFunctionOptions{
		Logger:         suite.Logger,
		FunctionConfig: createAFunctionResult.UpdatedFunctionConfig,
	}

	// deploy the 3rd function based on functionA image name
	suite.DeployFunction(createFunctionFromImageOptions, func(deployResult *platform.CreateFunctionResult) bool {
		request := &httpsuite.Request{
			RequestMethod:        "POST",
			RequestBody:          "/",
			ExpectedResponseBody: fmt.Sprintf("%s\n", functionAResponse),
			RequestPort:          deployResult.Port,
		}
		suite.SendRequestVerifyResponse(request)
		suite.NotEmpty(deployResult.Image)

		// update function to take image from functionB
		createFunctionFromImageOptions.FunctionConfig.Meta.Name = createBFunctionResult.UpdatedFunctionConfig.Meta.Name
		createFunctionFromImageOptions.FunctionConfig.Spec.Handler = createBFunctionResult.UpdatedFunctionConfig.Spec.Handler
		createFunctionFromImageOptions.FunctionConfig.Spec.Image = createBFunctionResult.Image
		createFunctionFromImageOptions.FunctionConfig.Spec.Env = []v1.EnvVar{
			{Name: "MESSAGE", Value: functionBResponse},
		}

		// redeploy function, use functionB image - its response should be equal to functionB
		redeployResults := suite.DeployFunctionAndRequest(createFunctionFromImageOptions,
			&httpsuite.Request{
				RequestMethod:        "POST",
				ExpectedResponseBody: fmt.Sprintf("%s\n", functionBResponse),
			})
		return suite.NotEqual(deployResult.Image, redeployResults.Image)
	})
}

func (suite *testSuite) TestBuildFuncFromRemoteArchiveRedeploy() {
	createFunctionOptions := &platform.CreateFunctionOptions{
		Logger: suite.Logger,
		FunctionConfig: functionconfig.Config{
			Meta: functionconfig.Meta{
				Name:      "build-from-local",
				Namespace: "default",
			},
			Spec: functionconfig.Spec{
				Env: []v1.EnvVar{
					{
						Name:  "MANIPULATION_KIND",
						Value: "reverse",
					},
				},
				Handler: "string-manipulator:handler",
				Runtime: "python",
				Build: functionconfig.Build{
					CodeEntryAttributes: map[string]interface{}{
						"workDir": "/nuclio-templates-master/string-manipulator",
					},
					Path: "https://github.com/nuclio/nuclio-templates/archive/master.zip",
				},
			},
		},
	}
	deployResult := suite.DeployFunctionAndRequest(createFunctionOptions,
		&httpsuite.Request{
			RequestMethod:        "POST",
			RequestBody:          "abcd",
			ExpectedResponseBody: "dcba",
		})

	suite.Equal(deployResult.CreateFunctionBuildResult.UpdatedFunctionConfig.Spec.Build.CodeEntryType, "archive")

	// validate that when redeploying it works and the function uses another image than before
	redeployFunctionOptions := &platform.CreateFunctionOptions{
		Logger:         suite.Logger,
		FunctionConfig: deployResult.UpdatedFunctionConfig,
	}
	redeployResult := suite.DeployFunctionAndRequest(redeployFunctionOptions,
		&httpsuite.Request{
			RequestMethod:        "POST",
			RequestBody:          "abcd",
			ExpectedResponseBody: "dcba",
		})

	suite.NotEqual(deployResult.Image, redeployResult.Image)
}

func (suite *testSuite) TestBuildFuncFromLocalArchiveRedeployUsesSameImage() {
	createFunctionOptions := &platform.CreateFunctionOptions{
		Logger: suite.Logger,
		FunctionConfig: functionconfig.Config{
			Meta: functionconfig.Meta{
				Name:      "build-from-local",
				Namespace: "default",
			},
			Spec: functionconfig.Spec{
				Handler: "main:handler",
				Runtime: "python",
				Build: functionconfig.Build{
					CodeEntryAttributes: map[string]interface{}{
						"workDir": "/funcs/my-python-func",
					},
					Path: path.Join(suite.GetNuclioSourceDir(), "pkg", "processor", "build", "test", "test_funcs.zip"),
				},
			},
		},
	}
	deployResult := suite.DeployFunctionAndRequest(createFunctionOptions,
		&httpsuite.Request{
			RequestMethod:        "GET",
			ExpectedResponseBody: "hello world",
		})

	suite.Equal(deployResult.CreateFunctionBuildResult.UpdatedFunctionConfig.Spec.Build.CodeEntryType, "image")

	// validate that when redeploying it works and the function uses the same image as before
	redeployFunctionOptions := &platform.CreateFunctionOptions{
		Logger:         suite.Logger,
		FunctionConfig: deployResult.UpdatedFunctionConfig,
	}
	redeployResult := suite.DeployFunctionAndRequest(redeployFunctionOptions,
		&httpsuite.Request{
			RequestMethod:        "GET",
			ExpectedResponseBody: "hello world",
		})

	suite.Equal(deployResult.Image, redeployResult.Image)
}

func (suite *testSuite) TestGenerateProcessorDockerfile() {
	newPlatform, err := local.NewPlatform(suite.TestSuite.Ctx, suite.Logger, &platformconfig.Config{}, "")
	suite.Require().NoErrorf(err, "Instantiating Platform failed: %s", err)

	builder, err := build.NewBuilder(suite.Logger, newPlatform, nil)
	suite.Require().NoErrorf(err, "Instantiating Builder failed: %s", err)

	// all elements, health check required
	suite.generateDockerfileAndVerify(builder, true, &runtime.ProcessorDockerfileInfo{
		BaseImage: "baseImage",
		OnbuildArtifacts: []runtime.Artifact{
			{
				Name:  "onbuild-1",
				Image: "onbuildImage",
				Paths: map[string]string{
					"onbuildLocal1": "onbuildImage1",
					"onbuildLocal2": "onbuildImage2",
				},
			},
			{
				Name:  "uhttpc-1",
				Image: "quay.io/nuclio/uhttpc:0.0.1-amd64",
				Paths: map[string]string{
					"/home/nuclio/bin/uhttpc": "/usr/local/bin/uhttpc",
				},
				ExternalImage: true,
			},
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
	}, `# Multistage builds
# From the base image
FROM baseImage
# Old(er) Docker support - must use all build args
ARG NUCLIO_LABEL
ARG NUCLIO_ARCH
ARG NUCLIO_BUILD_LOCAL_HANDLER_DIR
# Run the pre-copy directives
preCopyKind1 preCopyValue1
preCopyKind2 preCopyValue2
# Copy required objects from the suppliers
COPY artifacts/onbuildLocal1 onbuildImage1
COPY artifacts/onbuildLocal2 onbuildImage2
COPY artifacts/uhttpc /usr/local/bin/uhttpc
COPY imageLocal1 imageImage1
COPY imageLocal2 imageImage2
# Readiness probe
HEALTHCHECK --interval=1s --timeout=3s CMD /usr/local/bin/uhttpc --url http://127.0.0.1:8082/ready || exit 1
# Run the post-copy directives
postCopyKind1 postCopyValue1
postCopyKind2 postCopyValue2
# Run processor with configuration and platform configuration
CMD [ "processor" ]`)

	// all elements, health check not required
	suite.generateDockerfileAndVerify(builder, false, &runtime.ProcessorDockerfileInfo{
		BaseImage: "baseImage",
		OnbuildArtifacts: []runtime.Artifact{
			{
				Name:  "onbuild-1",
				Image: "onbuildImage",
				Paths: map[string]string{
					"onbuildLocal1": "onbuildImage1",
					"onbuildLocal2": "onbuildImage2",
				},
			},
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
	}, `# Multistage builds
# From the base image
FROM baseImage
# Old(er) Docker support - must use all build args
ARG NUCLIO_LABEL
ARG NUCLIO_ARCH
ARG NUCLIO_BUILD_LOCAL_HANDLER_DIR
# Run the pre-copy directives
preCopyKind1 preCopyValue1
preCopyKind2 preCopyValue2
# Copy required objects from the suppliers
COPY artifacts/onbuildLocal1 onbuildImage1
COPY artifacts/onbuildLocal2 onbuildImage2
COPY imageLocal1 imageImage1
COPY imageLocal2 imageImage2
# Run the post-copy directives
postCopyKind1 postCopyValue1
postCopyKind2 postCopyValue2
# Run processor with configuration and platform configuration
CMD [ "processor" ]`)
}

func (suite *testSuite) TestGenerateKanikoProcessorDockerfile() {

	containerBuilderConfiguration := &containerimagebuilderpusher.ContainerBuilderConfiguration{
		Kind: "kaniko",
	}

	containerBuilder, err := containerimagebuilderpusher.NewKaniko(suite.Logger, nil, containerBuilderConfiguration)
	if err != nil {
		suite.Fail("Instantiating kaniko builder failed:", err)
	}

	newPlatform := &kube.Platform{
		Platform: &abstract.Platform{
			ContainerBuilder: containerBuilder,
		},
	}

	builder, err := build.NewBuilder(suite.Logger, newPlatform, nil)
	if err != nil {
		suite.Fail("Instantiating Builder failed:", err)
	}

	// all elements, health check required
	suite.generateDockerfileAndVerify(builder, true, &runtime.ProcessorDockerfileInfo{
		BaseImage: "baseImage",
		OnbuildArtifacts: []runtime.Artifact{
			{
				Name:  "onbuild-1",
				Image: "onbuildImage",
				Paths: map[string]string{
					"onbuildLocal1": "onbuildImage1",
					"onbuildLocal2": "onbuildImage2",
				},
			},
			{
				Name:  "uhttpc-1",
				Image: "quay.io/nuclio/uhttpc:0.0.1-amd64",
				Paths: map[string]string{
					"/home/nuclio/bin/uhttpc": "/usr/local/bin/uhttpc",
				},
				ExternalImage: true,
			},
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
	}, `# Multistage builds
FROM onbuildImage AS onbuild-1
ARG NUCLIO_LABEL
ARG NUCLIO_ARCH
# From the base image
FROM baseImage
# Old(er) Docker support - must use all build args
ARG NUCLIO_LABEL
ARG NUCLIO_ARCH
ARG NUCLIO_BUILD_LOCAL_HANDLER_DIR
# Run the pre-copy directives
preCopyKind1 preCopyValue1
preCopyKind2 preCopyValue2
# Copy required objects from the suppliers
COPY --from=onbuild-1 onbuildLocal1 onbuildImage1
COPY --from=onbuild-1 onbuildLocal2 onbuildImage2
COPY --from=quay.io/nuclio/uhttpc:0.0.1-amd64 /home/nuclio/bin/uhttpc /usr/local/bin/uhttpc
COPY imageLocal1 imageImage1
COPY imageLocal2 imageImage2
# Readiness probe
HEALTHCHECK --interval=1s --timeout=3s CMD /usr/local/bin/uhttpc --url http://127.0.0.1:8082/ready || exit 1
# Run the post-copy directives
postCopyKind1 postCopyValue1
postCopyKind2 postCopyValue2
# Run processor with configuration and platform configuration
CMD [ "processor" ]`)

	// all elements, health check not required
	suite.generateDockerfileAndVerify(builder, false, &runtime.ProcessorDockerfileInfo{
		BaseImage: "baseImage",
		OnbuildArtifacts: []runtime.Artifact{
			{
				Name:  "onbuild-1",
				Image: "onbuildImage",
				Paths: map[string]string{
					"onbuildLocal1": "onbuildImage1",
					"onbuildLocal2": "onbuildImage2",
				},
			},
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
	}, `# Multistage builds
FROM onbuildImage AS onbuild-1
ARG NUCLIO_LABEL
ARG NUCLIO_ARCH
# From the base image
FROM baseImage
# Old(er) Docker support - must use all build args
ARG NUCLIO_LABEL
ARG NUCLIO_ARCH
ARG NUCLIO_BUILD_LOCAL_HANDLER_DIR
# Run the pre-copy directives
preCopyKind1 preCopyValue1
preCopyKind2 preCopyValue2
# Copy required objects from the suppliers
COPY --from=onbuild-1 onbuildLocal1 onbuildImage1
COPY --from=onbuild-1 onbuildLocal2 onbuildImage2
COPY imageLocal1 imageImage1
COPY imageLocal2 imageImage2
# Run the post-copy directives
postCopyKind1 postCopyValue1
postCopyKind2 postCopyValue2
# Run processor with configuration and platform configuration
CMD [ "processor" ]`)
}

func (suite *testSuite) generateDockerfileAndVerify(builder *build.Builder,
	healthCheckRequired bool,
	dockerfileInfo *runtime.ProcessorDockerfileInfo,
	expectedDockerfile string) {

	dockerfileContents, err := builder.GenerateDockerfileContents(dockerfileInfo.BaseImage,
		dockerfileInfo.OnbuildArtifacts,
		dockerfileInfo.ImageArtifactPaths,
		dockerfileInfo.Directives,
		healthCheckRequired,
		dockerfileInfo.BuildArgs)

	dockerfileContents = common.RemoveEmptyLines(dockerfileContents)

	suite.Require().NoError(err)
	suite.Require().Equal(expectedDockerfile, common.RemoveEmptyLines(dockerfileContents))
}

func (suite *testSuite) createShellFunctionFromSourceCode(functionName string,
	sourceCode string,
	request *httpsuite.Request) *platform.CreateFunctionResult {

	if functionName == "" {

		// fallback to a random name
		functionName = xid.New().String()
	}
	createFunctionOptions := &platform.CreateFunctionOptions{
		Logger: suite.Logger,
		FunctionConfig: functionconfig.Config{
			Meta: functionconfig.Meta{
				Name:      functionName,
				Namespace: "default",
			},
			Spec: functionconfig.Spec{
				Handler: fmt.Sprintf("%s.sh", functionName),
				Runtime: "shell",
				Build: functionconfig.Build{
					FunctionSourceCode: base64.StdEncoding.EncodeToString([]byte(sourceCode)),
				},
			},
		},
	}
	return suite.DeployFunctionAndRequest(createFunctionOptions, request)
}

func (suite *testSuite) createShellFunctionWithResponse(responseMessage string) *platform.CreateFunctionResult {
	functionName := xid.New().String()
	functionSourceCode := fmt.Sprintf(`
# @nuclio.configure
#
# function.yaml:
#   metadata:
#     name: %s
#     namespace: default
#   spec:
#     env:
#     - name: MESSAGE
#       value: %s
echo ${MESSAGE}
`, functionName, responseMessage)

	return suite.createShellFunctionFromSourceCode(functionName, functionSourceCode,
		&httpsuite.Request{
			RequestMethod:        "POST",
			ExpectedResponseBody: fmt.Sprintf("%s\n", responseMessage),
		})
}

func TestIntegrationSuite(t *testing.T) {
	if testing.Short() {
		return
	}

	suite.Run(t, new(testSuite))
}
