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
	"context"
	"net/http"
	"path"

	"github.com/nuclio/nuclio/pkg/platform"
	"github.com/nuclio/nuclio/pkg/processor/trigger/http/test/suite"
)

type FunctionInfo struct {
	Path []string
	Handler string
	Runtime string
	Skip bool
}

type RuntimeSuite interface {

	GetFunctionInfo(name string) FunctionInfo
}


type TestSuite struct {
	httpsuite.TestSuite
	RuntimeSuite RuntimeSuite
}

func (suite *TestSuite) GetProcessorBuildDir() string {
	return path.Join(suite.GetNuclioSourceDir(), "pkg", "processor", "build", "runtime")
}

func (suite *TestSuite) GetTestFunctionsDir() string {
	return path.Join(suite.GetNuclioSourceDir(), "test", "_functions")
}

func (suite *TestSuite) TestBuildFile() {
	suite.DeployFunctionAndRequest(suite.getDeployOptions("reverser"),
		&httpsuite.Request{
			RequestMethod:        "POST",
			RequestBody:          "abcdef",
			ExpectedResponseBody: "fedcba",
		})
}

func (suite *TestSuite) TestBuildDir() {
	suite.DeployFunctionAndRequest(suite.getDeployOptions("reverser-dir"),
		&httpsuite.Request{
			RequestMethod:        "POST",
			RequestBody:          "abcdef",
			ExpectedResponseBody: "fedcba",
		})
}

func (suite *TestSuite) TestBuildURL() {
	deployOptions := suite.getDeployOptions("reverser")
	pathToFunction := "/some/path/to/function/" + path.Base(deployOptions.FunctionConfig.Spec.Build.Path)

	// start an HTTP server to serve the reverser py
	// TODO: needs to be made unique (find a free port)
	httpServer := HTTPFileServer{}
	httpServer.Start(":7777",
		deployOptions.FunctionConfig.Spec.Build.Path,
		pathToFunction)

	defer httpServer.Shutdown(context.TODO())

	deployOptions.FunctionConfig.Spec.Build.Path = "http://localhost:7777" + pathToFunction

	suite.DeployFunctionAndRequest(deployOptions,
		&httpsuite.Request{
			RequestMethod:        "POST",
			RequestBody:          "abcdef",
			ExpectedResponseBody: "fedcba",
		})
}

func (suite *TestSuite) TestBuildDirWithFunctionConfig() {
	deployOptions := suite.getDeployOptions("json-parser-with-function-config")

	suite.DeployFunctionAndRequest(deployOptions,
		&httpsuite.Request{
			RequestBody:          `{"a": 100, "return_this": "returned value"}`,
			ExpectedResponseBody: "returned value",
		})
}

func (suite *TestSuite) TestBuildDirWithInlineFunctionConfig() {
	deployOptions := suite.getDeployOptions("json-parser-with-inline-function-config")

	suite.DeployFunctionAndRequest(deployOptions,
		&httpsuite.Request{
			RequestBody:          `{"a": 100, "return_this": "returned value"}`,
			ExpectedResponseBody: "returned value",
		})
}

func (suite *TestSuite) getDeployOptions(functionName string) *platform.DeployOptions {
	functionInfo := suite.RuntimeSuite.GetFunctionInfo(functionName)

	if functionInfo.Skip {
		suite.T().Skip()
	}

	deployOptions := suite.GetDeployOptions(functionName,
		path.Join(functionInfo.Path...))

	deployOptions.FunctionConfig.Spec.Handler = functionInfo.Handler
	deployOptions.FunctionConfig.Spec.Runtime = functionInfo.Runtime

	return deployOptions
}

//
// HTTP server to test URL fetch
//

type HTTPFileServer struct {
	http.Server
}

func (hfs *HTTPFileServer) Start(addr string, localPath string, pattern string) {
	hfs.Addr = addr

	http.HandleFunc(pattern, func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, localPath)
	})

	go hfs.ListenAndServe()
}
