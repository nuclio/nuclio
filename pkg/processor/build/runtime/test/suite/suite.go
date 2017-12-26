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
	"io/ioutil"
	"net/http"
	"path"

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

func (suite *TestSuite) TestBuildDirWithRuntimeFromFunctionConfig() {
	deployOptions := suite.getDeployOptions("json-parser-with-function-config")

	deployOptions.FunctionConfig.Spec.Runtime = ""

	suite.DeployFunctionAndRequest(deployOptions,
		&httpsuite.Request{
			RequestBody:          `{"a": 100, "return_this": "returned value"}`,
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

func (suite *TestSuite) TestBuildCustomImageName() {
	deployOptions := suite.getDeployOptions("reverser")

	// update image name
	deployOptions.FunctionConfig.Spec.Build.ImageName = "myname" + suite.TestID

	deployResult := suite.DeployFunctionAndRequest(deployOptions,
		&httpsuite.Request{
			RequestBody:          "abcdef",
			ExpectedResponseBody: "fedcba",
		})

	suite.Require().Equal(deployOptions.FunctionConfig.Spec.Build.ImageName+":latest", deployResult.ImageName)
}

func (suite *TestSuite) compressAndDeployFunctionFromURL(archiveExtension string,
	compressor func(string, []string) error) {

	deployOptions := suite.getDeployOptionsDir("reverser")

	archivePath := suite.createFunctionArchive(deployOptions.FunctionConfig.Spec.Build.Path,
		archiveExtension,
		compressor)

	pathToFunction := "/some/path/to/function/" + path.Base(archivePath)

	// start an HTTP server to serve the reverser py
	// TODO: needs to be made unique (find a free port)
	httpServer := HTTPFileServer{}
	httpServer.Start(":7777",
		archivePath,
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

func (suite *TestSuite) getDeployOptionsDir(functionName string) *platform.DeployOptions {
	deployOptions := suite.getDeployOptions(functionName)

	deployOptions.FunctionConfig.Spec.Build.Path = path.Dir(deployOptions.FunctionConfig.Spec.Build.Path)

	return deployOptions
}

func (suite *TestSuite) compressAndDeployFunction(archiveExtension string, compressor func(string, []string) error) {
	deployOptions := suite.getDeployOptionsDir("reverser")

	archivePath := suite.createFunctionArchive(deployOptions.FunctionConfig.Spec.Build.Path,
		archiveExtension,
		compressor)

	// set the path to the zip
	deployOptions.FunctionConfig.Spec.Build.Path = archivePath

	suite.DeployFunctionAndRequest(deployOptions,
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
