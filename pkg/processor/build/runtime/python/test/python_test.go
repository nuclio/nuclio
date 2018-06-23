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

package test

import (
	"encoding/base64"
	"fmt"
	"testing"
	"time"

	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/platform"
	"github.com/nuclio/nuclio/pkg/processor/build/runtime/test/suite"
	"github.com/nuclio/nuclio/pkg/processor/trigger/http/test/suite"
	"github.com/nuclio/nuclio/test/httpsrv"
	"github.com/rs/xid"

	"github.com/stretchr/testify/suite"
)

type testSuite struct {
	buildsuite.TestSuite
	runtime string
}

func newTestSuite(runtime string) *testSuite {
	return &testSuite{
		runtime: runtime,
	}
}

func (suite *testSuite) SetupSuite() {
	suite.TestSuite.SetupSuite()

	suite.TestSuite.RuntimeSuite = suite
}

func (suite *testSuite) TestBuildPy2() {
	createFunctionOptions := suite.GetDeployOptions("printer",
		suite.GetFunctionPath(suite.GetTestFunctionsDir(), "python", "py2-printer"))

	createFunctionOptions.FunctionConfig.Spec.Runtime = "python:2.7"
	createFunctionOptions.FunctionConfig.Spec.Handler = "printer:handler"

	suite.DeployFunctionAndRequest(createFunctionOptions,
		&httpsuite.Request{
			RequestMethod:        "POST",
			RequestBody:          "",
			ExpectedResponseBody: "printed",
		})
}

func (suite *testSuite) GetFunctionInfo(functionName string) buildsuite.FunctionInfo {
	functionInfo := buildsuite.FunctionInfo{
		Runtime: suite.runtime,
	}

	switch functionName {

	case "reverser":
		functionInfo.Path = []string{suite.GetTestFunctionsDir(), "common", "reverser", "python", "reverser.py"}
		functionInfo.Handler = "reverser:handler"

	case "json-parser-with-function-config":
		functionInfo.Path = []string{suite.GetTestFunctionsDir(), "common", "json-parser-with-function-config", "python"}

	case "json-parser-with-inline-function-config":
		functionInfo.Path = []string{suite.GetTestFunctionsDir(), "common", "json-parser-with-inline-function-config", "python", "parser.py"}

	case "long-initialization":
		functionInfo.Path = []string{suite.GetTestFunctionsDir(), "common", "long-initialization", "python", "sleepy.py"}

	default:
		suite.Logger.InfoWith("Test skipped", "functionName", functionName)

		functionInfo.Skip = true
	}

	return functionInfo
}

func (suite *testSuite) TestDockerCacheUtilized() {
	firstFilePath := "/first-file.txt"
	secondFilePath := "/second-file.txt"
	firstFilePattern := "first-file"
	secondFilePattern := "second-file"
	serverAddress := "10.0.0.12:7777"

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

	readinessTimeout := 5 * time.Second

	createFunctionOptions := &platform.CreateFunctionOptions{
		Logger:           suite.Logger,
		FunctionConfig:   *functionconfig.NewConfig(),
		ReadinessTimeout: &readinessTimeout,
	}

	createFunctionOptions.FunctionConfig.Meta.Name = "cache-test"
	createFunctionOptions.FunctionConfig.Spec.Runtime = "python:3.6"
	createFunctionOptions.FunctionConfig.Spec.Handler = "cachetest:handler"
	createFunctionOptions.FunctionConfig.Spec.Build.TempDir = suite.CreateTempDir()
	createFunctionOptions.FunctionConfig.Spec.Build.Commands = []string{

		// install curl
		"apk --update --no-cache add curl",

		// fetch first file
		fmt.Sprintf("curl -L %s/%s --output %s", serverAddress, firstFilePattern, firstFilePath),

		// indicate that commands from here on out should execute _after_ the copy of artifacts
		"@nuclio.postCopy",

		// fetch second file
		fmt.Sprintf("curl -L %s/%s --output %s", serverAddress, secondFilePattern, secondFilePath),
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
	httpServer.Stop()

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
	httpServer.Stop()

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
	httpServer.Stop()
}

func TestIntegrationSuite(t *testing.T) {
	if testing.Short() {
		return
	}

	//suite.Run(t, newTestSuite("python"))
	//suite.Run(t, newTestSuite("python:2.7"))
	suite.Run(t, newTestSuite("python:3.6"))
}
