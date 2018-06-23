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
	"github.com/nuclio/nuclio/pkg/processor/trigger/http/test/suite"
	"github.com/nuclio/nuclio/test/httpsrv"

	"github.com/rs/xid"
	"github.com/stretchr/testify/suite"
)

type testSuite struct {
	httpsuite.TestSuite
}

func newTestSuite() *testSuite {
	return &testSuite{}
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

	suite.Run(t, newTestSuite())
}
