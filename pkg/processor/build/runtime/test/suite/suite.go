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
	"encoding/base64"
	"io/ioutil"
	"net/http"
	"path"
	"time"

	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/platform"
	"github.com/nuclio/nuclio/pkg/processor/trigger/http/test/suite"

	"github.com/mholt/archiver"
)

type FunctionInfo struct {
	Path    []string
	Handler string
	Runtime string
	Skip    bool
}

type RuntimeSuite interface {
	GetFunctionInfo(functionName string) FunctionInfo
}

type archiveInfo struct {
	extension  string
	compressor func(string, []string) error
}

type TestSuite struct {
	httpsuite.TestSuite
	RuntimeSuite RuntimeSuite
	archiveInfos []archiveInfo
}

func (suite *TestSuite) SetupSuite() {
	suite.TestSuite.SetupSuite()

	suite.archiveInfos = []archiveInfo{
		{".zip", archiver.Zip.Make},
		{".tar.gz", archiver.TarGz.Make},
	}
}

func (suite *TestSuite) GetProcessorBuildDir() string {
	return path.Join(suite.GetNuclioSourceDir(), "pkg", "processor", "build", "runtime")
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
	suite.DeployFunctionAndRequest(suite.getDeployOptionsDir("reverser"),
		&httpsuite.Request{
			RequestMethod:        "POST",
			RequestBody:          "abcdef",
			ExpectedResponseBody: "fedcba",
		})
}

func (suite *TestSuite) TestBuildURL() {
}

func (suite *TestSuite) TestBuildDirWithFunctionConfig() {
	createFunctionOptions := suite.getDeployOptions("json-parser-with-function-config")

	suite.DeployFunctionAndRequest(createFunctionOptions,
		&httpsuite.Request{
			RequestBody:          `{"a": 100, "return_this": "returned value"}`,
			RequestHeaders:       map[string]interface{}{"Content-Type": "application/json"},
			ExpectedResponseBody: "returned value",
		})
}

func (suite *TestSuite) TestBuildDirWithInlineFunctionConfig() {
	createFunctionOptions := suite.getDeployOptions("json-parser-with-inline-function-config")

	suite.DeployFunctionAndRequest(createFunctionOptions,
		&httpsuite.Request{
			RequestBody:          `{"a": 100, "return_this": "returned value"}`,
			RequestHeaders:       map[string]interface{}{"Content-Type": "application/json"},
			ExpectedResponseBody: "returned value",
		})
}

func (suite *TestSuite) TestBuildDirWithRuntimeFromFunctionConfig() {
	createFunctionOptions := suite.getDeployOptions("json-parser-with-function-config")

	createFunctionOptions.FunctionConfig.Spec.Runtime = ""

	suite.DeployFunctionAndRequest(createFunctionOptions,
		&httpsuite.Request{
			RequestBody:          `{"a": 100, "return_this": "returned value"}`,
			RequestHeaders:       map[string]interface{}{"Content-Type": "application/json"},
			ExpectedResponseBody: "returned value",
		})
}

func (suite *TestSuite) TestBuildArchive() {
	for _, archiveInfo := range suite.archiveInfos {
		suite.compressAndDeployFunction(archiveInfo.extension, archiveInfo.compressor)
	}
}

func (suite *TestSuite) TestBuildArchiveFromURL() {
	for _, archiveInfo := range suite.archiveInfos {
		suite.compressAndDeployFunctionFromURL(archiveInfo.extension, archiveInfo.compressor)
	}
}

func (suite *TestSuite) TestBuildFuncFromSourceString() {
	createFunctionOptions := suite.getDeployOptions("reverser")

	// Java "source" is a jar file, and it it'll be a .java file it must be named in the same name as the class
	// Skip for now
	if createFunctionOptions.FunctionConfig.Spec.Runtime == "java" {
		suite.T().Skip("Java runtime not supported")
		return
	}

	functionSourceCode, err := ioutil.ReadFile(createFunctionOptions.FunctionConfig.Spec.Build.Path)
	suite.Assert().NoError(err)

	createFunctionOptions.FunctionConfig.Spec.Build.FunctionSourceCode = base64.StdEncoding.EncodeToString(functionSourceCode)
	createFunctionOptions.FunctionConfig.Spec.Build.Path = ""

	switch createFunctionOptions.FunctionConfig.Spec.Runtime {
	case "golang":
		createFunctionOptions.FunctionConfig.Spec.Handler = "handler:Reverse"
	case "shell":
		createFunctionOptions.FunctionConfig.Spec.Handler = "handler.sh:main"
	case "dotnetcore":
		createFunctionOptions.FunctionConfig.Spec.Handler = "nuclio:reverser"
	default:
		createFunctionOptions.FunctionConfig.Spec.Handler = "handler:handler"
	}

	suite.DeployFunctionAndRequest(createFunctionOptions,
		&httpsuite.Request{
			RequestBody:          "abcdef",
			ExpectedResponseBody: "fedcba",
		})
}

func (suite *TestSuite) TestBuildCustomImage() {
	createFunctionOptions := suite.getDeployOptions("reverser")

	// update image name
	createFunctionOptions.FunctionConfig.Spec.Build.Image = "myname" + suite.TestID

	deployResult := suite.DeployFunctionAndRequest(createFunctionOptions,
		&httpsuite.Request{
			RequestBody:          "abcdef",
			ExpectedResponseBody: "fedcba",
		})

	suite.Require().Equal(createFunctionOptions.FunctionConfig.Spec.Build.Image+":latest", deployResult.Image)
}

func (suite *TestSuite) TestBuildCustomHTTPPort() {
	httpPort := 31000

	createFunctionOptions := suite.getDeployOptions("reverser")

	createFunctionOptions.FunctionConfig.Spec.Triggers = map[string]functionconfig.Trigger{
		"http": {
			Kind: "http",
			Attributes: map[string]interface{}{
				"port": httpPort,
			},
		},
	}

	suite.DeployFunctionAndRequest(createFunctionOptions,
		&httpsuite.Request{
			RequestBody:          "abcdef",
			ExpectedResponseBody: "fedcba",
			RequestPort:          httpPort,
		})
}

func (suite *TestSuite) TestBuildSpecifyingFunctionConfig() {
	createFunctionOptions := suite.getDeployOptions("json-parser-with-function-config")

	createFunctionOptions.FunctionConfig.Meta.Name = ""
	createFunctionOptions.FunctionConfig.Spec.Runtime = ""

	suite.DeployFunctionAndRequest(createFunctionOptions,
		&httpsuite.Request{
			RequestBody:          `{"a": 100, "return_this": "returned value"}`,
			RequestHeaders:       map[string]interface{}{"Content-Type": "application/json"},
			ExpectedResponseBody: "returned value",
		})
}

func (suite *TestSuite) TestBuildLongInitialization() {

	// long-initialization functions have a 5-second sleep on load
	createFunctionOptions := suite.getDeployOptions("long-initialization")

	// allow the function up to 10 seconds to be ready
	timeout := 10 * time.Second
	createFunctionOptions.ReadinessTimeout = &timeout

	suite.DeployFunctionAndRequest(createFunctionOptions,
		&httpsuite.Request{
			ExpectedResponseBody: "Good morning",
		})
}

func (suite *TestSuite) TestBuildLongInitializationReadinessTimeoutReached() {

	// long-initialization functions have a 5-second sleep on load
	createFunctionOptions := suite.getDeployOptions("long-initialization")

	// allow them less time than that to become ready, expect deploy to fail
	timeout := 3 * time.Second
	createFunctionOptions.ReadinessTimeout = &timeout

	suite.DeployFunctionAndExpectError(createFunctionOptions, "Function wasn't ready in time")

	// since the function does actually get deployed (just not ready in time), we need to delete it
	err := suite.Platform.DeleteFunction(&platform.DeleteFunctionOptions{
		FunctionConfig: createFunctionOptions.FunctionConfig,
	})
	suite.Require().NoError(err)

	// clean up the processor image we built
	err = suite.DockerClient.RemoveImage(createFunctionOptions.FunctionConfig.Spec.Image)
	suite.Require().NoError(err)
}

func (suite *TestSuite) compressAndDeployFunctionFromURL(archiveExtension string,
	compressor func(string, []string) error) {

	createFunctionOptions := suite.getDeployOptionsDir("reverser")

	archivePath := suite.createFunctionArchive(createFunctionOptions.FunctionConfig.Spec.Build.Path,
		archiveExtension,
		compressor)

	pathToFunction := "/some/path/to/function/" + path.Base(archivePath)

	// start an HTTP server to serve the reverser py
	// TODO: needs to be made unique (find a free port)
	httpServer := HTTPFileServer{}
	httpServer.Start(":7777",
		archivePath,
		pathToFunction)

	defer httpServer.Shutdown(context.TODO()) // nolint: errcheck

	createFunctionOptions.FunctionConfig.Spec.Build.Path = "http://localhost:7777" + pathToFunction

	suite.DeployFunctionAndRequest(createFunctionOptions,
		&httpsuite.Request{
			RequestMethod:        "POST",
			RequestBody:          "abcdef",
			ExpectedResponseBody: "fedcba",
		})

}

func (suite *TestSuite) getDeployOptionsDir(functionName string) *platform.CreateFunctionOptions {
	createFunctionOptions := suite.getDeployOptions(functionName)

	createFunctionOptions.FunctionConfig.Spec.Build.Path = path.Dir(createFunctionOptions.FunctionConfig.Spec.Build.Path)

	return createFunctionOptions
}

func (suite *TestSuite) compressAndDeployFunction(archiveExtension string, compressor func(string, []string) error) {
	createFunctionOptions := suite.getDeployOptionsDir("reverser")

	archivePath := suite.createFunctionArchive(createFunctionOptions.FunctionConfig.Spec.Build.Path,
		archiveExtension,
		compressor)

	// set the path to the zip
	createFunctionOptions.FunctionConfig.Spec.Build.Path = archivePath

	suite.DeployFunctionAndRequest(createFunctionOptions,
		&httpsuite.Request{
			RequestMethod:        "POST",
			RequestBody:          "abcdef",
			ExpectedResponseBody: "fedcba",
		})
}

func (suite *TestSuite) createFunctionArchive(functionDir string,
	archiveExtension string,
	compressor func(string, []string) error) string {

	// create a temp directory that will hold the archive
	archiveDir, err := ioutil.TempDir("", "build-zip-"+suite.TestID)
	suite.Require().NoError(err)

	// use the reverse function
	archivePath := path.Join(archiveDir, "reverser"+archiveExtension)

	functionFileInfos, err := ioutil.ReadDir(functionDir)
	suite.Require().NoError(err)

	var functionFileNames []string
	for _, functionFileInfo := range functionFileInfos {
		functionFileNames = append(functionFileNames,
			path.Join(functionDir, functionFileInfo.Name()))
	}

	// create the archive
	err = compressor(archivePath, functionFileNames)
	suite.Require().NoError(err)

	return archivePath
}

func (suite *TestSuite) getDeployOptions(functionName string) *platform.CreateFunctionOptions {
	functionInfo := suite.RuntimeSuite.GetFunctionInfo(functionName)

	if functionInfo.Skip {
		suite.T().Skip()
	}

	createFunctionOptions := suite.GetDeployOptions(functionName,
		path.Join(functionInfo.Path...))

	createFunctionOptions.FunctionConfig.Spec.Handler = functionInfo.Handler
	createFunctionOptions.FunctionConfig.Spec.Runtime = functionInfo.Runtime

	return createFunctionOptions
}

//
// HTTP server to test URL fetch
//

type HTTPFileServer struct {
	http.Server
}

func (hfs *HTTPFileServer) Start(addr string, localPath string, pattern string) {
	hfs.Addr = addr

	// create a new servemux
	serveMux := http.NewServeMux()
	serveMux.HandleFunc(pattern, func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, localPath)
	})

	hfs.Handler = serveMux

	go hfs.ListenAndServe() // nolint: errcheck
}
