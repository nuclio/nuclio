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
	"os"
	"path"
	"testing"

	"github.com/nuclio/nuclio/pkg/errors"
	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/platform"
	"github.com/nuclio/nuclio/pkg/processor/trigger/http/test/suite"
	"github.com/nuclio/nuclio/test/httpsrv"

	"github.com/rs/xid"
	"github.com/stretchr/testify/suite"
)

type testSuite struct {
	httpsuite.TestSuite
}

func (suite *testSuite) TestBuildFuncFromSourceWithInlineConfig() {
	createFunctionOptions := &platform.CreateFunctionOptions{
		Logger: suite.Logger,
	}

	functionSourceCode := `
# @nuclio.configure
#
# function.yaml:
#   metadata:
#     name: echo-foo-inline
#     namespace: default
#   spec:
#     env:
#     - name: MESSAGE
#       value: foo

echo $MESSAGE`

	createFunctionOptions.FunctionConfig.Spec.Handler = "echo-foo-inline.sh"
	createFunctionOptions.FunctionConfig.Spec.Runtime = "shell"
	createFunctionOptions.FunctionConfig.Spec.Build.Path = ""
	createFunctionOptions.FunctionConfig.Spec.Build.FunctionSourceCode = base64.StdEncoding.EncodeToString(
		[]byte(functionSourceCode))

	suite.DeployFunctionAndRequest(createFunctionOptions,
		&httpsuite.Request{
			RequestMethod:        "POST",
			RequestBody:          "",
			ExpectedResponseBody: "foo\n",
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
	defer os.Remove(tempFile.Name())

	// we *don't* want the contents of the temp file to appear in the function source code, because
	// the function source code is already populated
	tempFile.WriteString("Contents of temp file")

	createFunctionOptions.FunctionConfig.Meta.Name = "funcsource-test"
	createFunctionOptions.FunctionConfig.Meta.Namespace = "test"
	createFunctionOptions.FunctionConfig.Spec.Handler = "main:handler"
	createFunctionOptions.FunctionConfig.Spec.Runtime = "python:3.6"
	createFunctionOptions.FunctionConfig.Spec.Build.Path = tempFile.Name()
	createFunctionOptions.FunctionConfig.Spec.Build.FunctionSourceCode = functionSourceCode

	suite.DeployFunction(createFunctionOptions, func(deployResult *platform.CreateFunctionResult) bool {

		// get the function
		functions, err := suite.Platform.GetFunctions(&platform.GetFunctionsOptions{
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

func (suite *testSuite) TestBuildFunctionFromFileExpectSourceCodePopulated() {
	createFunctionOptions := suite.GetDeployOptions("reverser",
		path.Join(suite.GetNuclioSourceDir(), "test", "_functions", "common", "reverser", "python", "reverser.py"))

	createFunctionOptions.FunctionConfig.Spec.Runtime = "python:2.7"
	createFunctionOptions.FunctionConfig.Spec.Handler = "reverser:handler"

	suite.DeployFunction(createFunctionOptions, func(deployResult *platform.CreateFunctionResult) bool {

		// get the function
		functions, err := suite.Platform.GetFunctions(&platform.GetFunctionsOptions{
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
	createFunctionOptions := suite.GetDeployOptions("printer",
		path.Join(suite.GetNuclioSourceDir(), "test", "_functions", "python", "py2-printer"))

	createFunctionOptions.FunctionConfig.Spec.Runtime = "python:2.7"
	createFunctionOptions.FunctionConfig.Spec.Handler = "printer:handler"
	createFunctionOptions.FunctionConfig.Spec.Build.BaseImage = "python:3.6-jessie"

	createFunctionOptions.FunctionConfig.Spec.Build.Commands = append(createFunctionOptions.FunctionConfig.Spec.Build.Commands, "apt-get -qq update")
	createFunctionOptions.FunctionConfig.Spec.Build.Commands = append(createFunctionOptions.FunctionConfig.Spec.Build.Commands, "apt-get -qq install curl")

	suite.DeployFunctionAndRequest(createFunctionOptions,
		&httpsuite.Request{
			RequestMethod:        "POST",
			RequestBody:          "",
			ExpectedResponseBody: "printed",
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
	functionConfig.Spec.ReadinessTimeout = 5

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

	suite.Run(t, new(testSuite))
}
