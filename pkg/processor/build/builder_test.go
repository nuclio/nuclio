//go:build test_integration && test_local

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
limitations under the License.
*/

package build

import (
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"net/http"
	"path/filepath"
	"testing"

	"github.com/nuclio/nuclio/pkg/common"
	gitcommon "github.com/nuclio/nuclio/pkg/common/git"
	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/platform"
	mockplatform "github.com/nuclio/nuclio/pkg/platform/mock"

	"github.com/jarcoal/httpmock"
	"github.com/nuclio/errors"
	"github.com/nuclio/logger"
	"github.com/nuclio/zap"
	"github.com/rs/xid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

const (
	FunctionsArchiveFilePath = "test/test_funcs.zip"
)

//
// Test suite
//
type testSuite struct {
	suite.Suite
	logger       logger.Logger
	builder      *Builder
	testID       string
	mockS3Client *common.MockS3Client
	mockPlatform *mockplatform.Platform
}

// SetupSuite is called for suite setup
func (suite *testSuite) SetupSuite() {
	var err error

	suite.logger, err = nucliozap.NewNuclioZapTest("test")
	suite.Require().NoError(err)

	suite.mockS3Client = &common.MockS3Client{
		FilePath: FunctionsArchiveFilePath,
	}
	suite.mockPlatform = &mockplatform.Platform{}
}

// SetupTest is called before each test in the suite
func (suite *testSuite) SetupTest() {
	var err error
	suite.testID = xid.New().String()
	suite.builder, err = NewBuilder(suite.logger, suite.mockPlatform, suite.mockS3Client)
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
	tests := []struct {
		inputSourceCode    string
		expectedSourceCode string
	}{
		{"echo foo", "echo foo"},
		{"echo foo\n", "echo foo\n"},
		{"echo foo\r\n", "echo foo\n"},
	}
	suite.builder.options.FunctionConfig.Spec.Runtime = "shell"
	for _, test := range tests {
		err := suite.builder.createTempDir()
		suite.Require().NoError(err)
		encodedFunctionSourceCode := base64.StdEncoding.EncodeToString([]byte(test.inputSourceCode))
		suite.builder.options.FunctionConfig.Spec.Build.FunctionSourceCode = encodedFunctionSourceCode
		suite.builder.options.FunctionConfig.Spec.Build.Path = ""
		tempPath, err := suite.builder.writeFunctionSourceCodeToTempFile(encodedFunctionSourceCode)
		suite.Assert().NoError(err)
		suite.NotNil(tempPath)
		resultSourceCode, err := ioutil.ReadFile(tempPath)
		suite.Assert().NoError(err)
		suite.Assert().Equal(test.expectedSourceCode, string(resultSourceCode))
		err = suite.builder.cleanupTempDir()
		suite.Require().NoError(err)
	}
}

func (suite *testSuite) TestWriteFunctionSourceCodeToTempFileFailsOnUnknownExtension() {
	suite.builder.options.FunctionConfig.Spec.Runtime = "bar"
	suite.builder.options.FunctionConfig.Spec.Build.FunctionSourceCode = base64.StdEncoding.EncodeToString([]byte("echo foo"))
	suite.builder.options.FunctionConfig.Spec.Build.Path = ""

	err := suite.builder.createTempDir()
	suite.Assert().NoError(err)
	defer suite.builder.cleanupTempDir() // nolint: errcheck

	_, err = suite.builder.writeFunctionSourceCodeToTempFile(suite.builder.options.FunctionConfig.Spec.Build.FunctionSourceCode)
	suite.Assert().Error(err)
}

func (suite *testSuite) TestGetImage() {

	// user specified
	suite.builder.options.FunctionConfig.Spec.Build.Image = "userSpecified"
	imageName, err := suite.builder.getImage()
	suite.Require().NoError(err)
	suite.Require().Equal("userSpecified", imageName)

	// set function name and clear image name
	suite.builder.options.FunctionConfig.Meta.Name = "test"
	suite.builder.options.FunctionConfig.Spec.Build.Image = ""

	// registry has no repository - should see "nuclio/" as repository
	suite.builder.options.FunctionConfig.Spec.Build.Registry = "localhost:5000"
	suite.mockPlatform.On("RenderImageNamePrefixTemplate").Return("", nil).Once()
	imageName, err = suite.builder.getImage()
	suite.Require().NoError(err)
	suite.Require().Equal("nuclio/processor-test", imageName)

	// registry has a repository - should not see "nuclio/" as repository
	suite.builder.options.FunctionConfig.Spec.Build.Registry = "docker.io/foo"
	suite.mockPlatform.On("RenderImageNamePrefixTemplate").Return("", nil).Once()
	imageName, err = suite.builder.getImage()
	suite.Require().NoError(err)
	suite.Require().Equal("processor-test", imageName)

	// registry has a repository - should not see "nuclio/" as repository
	suite.builder.options.FunctionConfig.Spec.Build.Registry = "index.docker.io/foo"
	suite.mockPlatform.On("RenderImageNamePrefixTemplate").Return("", nil).Once()
	imageName, err = suite.builder.getImage()
	suite.Require().NoError(err)
	suite.Require().Equal("processor-test", imageName)

	// clear registry
	suite.builder.options.FunctionConfig.Spec.Build.Registry = "localhost:5000"

	// with prefix
	imageNamePrefix := "projectName-functionName-"
	suite.mockPlatform.On("RenderImageNamePrefixTemplate").Return(imageNamePrefix, nil).Once()
	imageName, err = suite.builder.getImage()
	suite.Require().NoError(err)
	suite.Require().Equal(fmt.Sprintf("nuclio/%sprocessor", imageNamePrefix), imageName)
}

func (suite *testSuite) TestMergeDirectives() {

	mergeDirectivesCases := []struct {
		first  map[string][]functionconfig.Directive
		second map[string][]functionconfig.Directive
		merged map[string][]functionconfig.Directive
	}{
		// first is full, second is empty
		{
			first: map[string][]functionconfig.Directive{
				"preCopy": {
					{Kind: "firstPreKind1", Value: "firstPreValue1"},
					{Kind: "firstPreKind2", Value: "firstPreValue2"},
				},
				"postCopy": {
					{Kind: "firstPostKind1", Value: "firstPostValue1"},
					{Kind: "firstPostKind2", Value: "firstPostValue2"},
				},
			},
			second: map[string][]functionconfig.Directive{},
			merged: map[string][]functionconfig.Directive{
				"preCopy": {
					{Kind: "firstPreKind1", Value: "firstPreValue1"},
					{Kind: "firstPreKind2", Value: "firstPreValue2"},
				},
				"postCopy": {
					{Kind: "firstPostKind1", Value: "firstPostValue1"},
					{Kind: "firstPostKind2", Value: "firstPostValue2"},
				},
			},
		},

		// first is partially empty, second is full
		{
			first: map[string][]functionconfig.Directive{},
			second: map[string][]functionconfig.Directive{
				"preCopy": {
					{Kind: "secondPreKind1", Value: "secondPreValue1"},
					{Kind: "secondPreKind2", Value: "secondPreValue2"},
				},
				"postCopy": {
					{Kind: "secondPostKind1", Value: "secondPostValue1"},
					{Kind: "secondPostKind2", Value: "secondPostValue2"},
				},
			},
			merged: map[string][]functionconfig.Directive{
				"preCopy": {
					{Kind: "secondPreKind1", Value: "secondPreValue1"},
					{Kind: "secondPreKind2", Value: "secondPreValue2"},
				},
				"postCopy": {
					{Kind: "secondPostKind1", Value: "secondPostValue1"},
					{Kind: "secondPostKind2", Value: "secondPostValue2"},
				},
			},
		},

		// first is partially full, second is full
		{
			first: map[string][]functionconfig.Directive{
				"postCopy": {
					{Kind: "firstPostKind1", Value: "firstPostValue1"},
					{Kind: "firstPostKind2", Value: "firstPostValue2"},
				},
			},
			second: map[string][]functionconfig.Directive{
				"preCopy": {
					{Kind: "secondPreKind1", Value: "secondPreValue1"},
					{Kind: "secondPreKind2", Value: "secondPreValue2"},
				},
				"postCopy": {
					{Kind: "secondPostKind1", Value: "secondPostValue1"},
					{Kind: "secondPostKind2", Value: "secondPostValue2"},
				},
			},
			merged: map[string][]functionconfig.Directive{
				"preCopy": {
					{Kind: "secondPreKind1", Value: "secondPreValue1"},
					{Kind: "secondPreKind2", Value: "secondPreValue2"},
				},
				"postCopy": {
					{Kind: "firstPostKind1", Value: "firstPostValue1"},
					{Kind: "firstPostKind2", Value: "firstPostValue2"},
					{Kind: "secondPostKind1", Value: "secondPostValue1"},
					{Kind: "secondPostKind2", Value: "secondPostValue2"},
				},
			},
		},

		// both are full
		{
			first: map[string][]functionconfig.Directive{
				"preCopy": {
					{Kind: "firstPreKind1", Value: "firstPreValue1"},
					{Kind: "firstPreKind2", Value: "firstPreValue2"},
				},
				"postCopy": {
					{Kind: "firstPostKind1", Value: "firstPostValue1"},
					{Kind: "firstPostKind2", Value: "firstPostValue2"},
				},
			},
			second: map[string][]functionconfig.Directive{
				"preCopy": {
					{Kind: "secondPreKind1", Value: "secondPreValue1"},
					{Kind: "secondPreKind2", Value: "secondPreValue2"},
				},
				"postCopy": {
					{Kind: "secondPostKind1", Value: "secondPostValue1"},
					{Kind: "secondPostKind2", Value: "secondPostValue2"},
				},
			},
			merged: map[string][]functionconfig.Directive{
				"preCopy": {
					{Kind: "firstPreKind1", Value: "firstPreValue1"},
					{Kind: "firstPreKind2", Value: "firstPreValue2"},
					{Kind: "secondPreKind1", Value: "secondPreValue1"},
					{Kind: "secondPreKind2", Value: "secondPreValue2"},
				},
				"postCopy": {
					{Kind: "firstPostKind1", Value: "firstPostValue1"},
					{Kind: "firstPostKind2", Value: "firstPostValue2"},
					{Kind: "secondPostKind1", Value: "secondPostValue1"},
					{Kind: "secondPostKind2", Value: "secondPostValue2"},
				},
			},
		},
	}

	for _, mergeDirectivesCase := range mergeDirectivesCases {

		// merge and compare
		suite.Require().Equal(mergeDirectivesCase.merged,
			suite.builder.mergeDirectives(mergeDirectivesCase.first, mergeDirectivesCase.second))
	}
}

func (suite *testSuite) TestCommandsToDirectives() {

	commandsToDirectivesCases := []struct {
		commands   []string
		directives map[string][]functionconfig.Directive
	}{
		// only pre commands
		{
			commands: []string{
				"preCommand1",
				"preCommand2",
				"preCommand3",
			},
			directives: map[string][]functionconfig.Directive{
				"preCopy": {
					{Kind: "RUN", Value: "preCommand1"},
					{Kind: "RUN", Value: "preCommand2"},
					{Kind: "RUN", Value: "preCommand3"},
				},
				"postCopy": {},
			},
		},
		// only post commands
		{
			commands: []string{
				"@nuclio.postCopy",
				"postCommand1",
				"postCommand2",
				"postCommand3",
			},
			directives: map[string][]functionconfig.Directive{
				"preCopy": {},
				"postCopy": {
					{Kind: "RUN", Value: "postCommand1"},
					{Kind: "RUN", Value: "postCommand2"},
					{Kind: "RUN", Value: "postCommand3"},
				},
			},
		},
		// pre and post
		{
			commands: []string{
				"preCommand1",
				"preCommand2",
				"@nuclio.postCopy",
				"postCommand1",
				"postCommand2",
				"postCommand3",
			},
			directives: map[string][]functionconfig.Directive{
				"preCopy": {
					{Kind: "RUN", Value: "preCommand1"},
					{Kind: "RUN", Value: "preCommand2"},
				},
				"postCopy": {
					{Kind: "RUN", Value: "postCommand1"},
					{Kind: "RUN", Value: "postCommand2"},
					{Kind: "RUN", Value: "postCommand3"},
				},
			},
		},
		// multi-line commands
		{
			commands: []string{
				"preCommand1 arg1 \\",
				"arg2 \\",
				"arg3",
				"preCommand2",
				"@nuclio.postCopy",
				"postCommand1",
			},
			directives: map[string][]functionconfig.Directive{
				"preCopy": {
					{Kind: "RUN", Value: "preCommand1 arg1 arg2 arg3"},
					{Kind: "RUN", Value: "preCommand2"},
				},
				"postCopy": {
					{Kind: "RUN", Value: "postCommand1"},
				},
			},
		},
	}

	for _, commandsToDirectivesCase := range commandsToDirectivesCases {
		directives, err := suite.builder.commandsToDirectives(commandsToDirectivesCase.commands)
		suite.Require().NoError(err)
		suite.Require().Equal(commandsToDirectivesCase.directives, directives)
	}
}

func (suite *testSuite) TestRenderDependantImageURL() {
	replacementURL := "replacement:port/sub"
	imageNameAndTag := "image-name:tag"

	// test render with replacement
	for _, testCase := range []struct {
		imageURL         string
		replacementURL   string
		expectedImageURL string
	}{
		{imageNameAndTag, "", imageNameAndTag},
		{"" + imageNameAndTag, replacementURL, replacementURL + "/" + imageNameAndTag},
		{"base/" + imageNameAndTag, replacementURL, replacementURL + "/" + imageNameAndTag},
		{"base/" + imageNameAndTag, replacementURL + "/", replacementURL + "/" + imageNameAndTag},
		{"base/sub/" + imageNameAndTag, replacementURL, replacementURL + "/" + imageNameAndTag},
	} {
		renderedImageURL, err := suite.builder.renderDependantImageURL(testCase.imageURL, testCase.replacementURL)
		suite.Require().NoError(err)
		suite.Require().Equal(testCase.expectedImageURL, renderedImageURL)
	}
}

func (suite *testSuite) TestValidateAndParseS3Attributes() {

	// return error when mandatory fields are missing
	badS3CodeEntryAttributes := map[string]interface{}{
		"s3Bucket": "mybucket",
	}
	_, err := suite.builder.validateAndParseS3Attributes(badS3CodeEntryAttributes)
	suite.Require().EqualError(err, "Mandatory field - 's3ItemKey' not given")

	// return error when some attribute is not of type string (all of them must be strings)
	badS3CodeEntryAttributes["s3ItemKey"] = 2
	_, err = suite.builder.validateAndParseS3Attributes(badS3CodeEntryAttributes)
	suite.Require().EqualError(err, "The given field - 's3ItemKey' is not of type string")

	// happy flow (map[string]interface{} -> map[string]string)
	goodS3CodeEntryAttributes := map[string]interface{}{
		"s3Bucket":          "my-bucket",
		"s3ItemKey":         "my-fold/my-item.zip",
		"s3Region":          "us-east-1",
		"s3AccessKeyId":     "myaccesskeyid",
		"s3SecretAccessKey": "mysecretaccesskey",
		"s3SessionToken":    "mys3sessiontoken",
	}
	expectedResult := map[string]string{
		"s3Bucket":          "my-bucket",
		"s3ItemKey":         "my-fold/my-item.zip",
		"s3Region":          "us-east-1",
		"s3AccessKeyId":     "myaccesskeyid",
		"s3SecretAccessKey": "mysecretaccesskey",
		"s3SessionToken":    "mys3sessiontoken",
	}
	res, err := suite.builder.validateAndParseS3Attributes(goodS3CodeEntryAttributes)
	suite.Require().NoError(err)
	suite.Require().Equal(expectedResult, res)
}

func (suite *testSuite) TestResolveFunctionPathRemoteCodeFile() {
	fileExtensions := []string{"py", "go", "cs", "java", "js", "sh", "rb"}
	for _, fileExtension := range fileExtensions {
		suite.testResolveFunctionPathRemoteCodeFile(fileExtension)
	}
}

func (suite *testSuite) TestResolveFunctionPathArchiveCodeEntry() {
	archiveFileURL := "http://some-address.com/test_function_archive"
	buildConfiguration := functionconfig.Build{
		CodeEntryType: ArchiveEntryType,
		Path:          archiveFileURL,
		CodeEntryAttributes: map[string]interface{}{
			"workDir": "/funcs/my-python-func",
		},
	}
	suite.testResolveFunctionPathArchive(buildConfiguration, archiveFileURL)
}

func (suite *testSuite) TestResolveFunctionPathNonExistingWorkDir() {
	archiveFileURL := "http://some-address.com/test_function_archive"
	buildConfiguration := functionconfig.Build{
		CodeEntryType: ArchiveEntryType,
		Path:          archiveFileURL,
		CodeEntryAttributes: map[string]interface{}{
			"workDir": "/non-existing-work-dir/fralalala",
		},
	}
	suite.testResolveFunctionPathArchiveBadWorkDir(buildConfiguration, archiveFileURL, string(common.WorkDirectoryDoesNotExist))
}

func (suite *testSuite) TestResolveFunctionPathNonStringWorkDir() {
	archiveFileURL := "http://some-address.com/test_function_archive"
	buildConfiguration := functionconfig.Build{
		CodeEntryType: ArchiveEntryType,
		Path:          archiveFileURL,
		CodeEntryAttributes: map[string]interface{}{
			"workDir": 213,
		},
	}
	suite.testResolveFunctionPathArchiveBadWorkDir(buildConfiguration, archiveFileURL, string(common.WorkDirectoryExpectedBeString))
}

func (suite *testSuite) TestResolveFunctionPathGithubCodeEntry() {
	archiveFileURL := "https://github.com/nuclio/my-func/archive/master.zip"
	buildConfiguration := functionconfig.Build{
		CodeEntryType: GithubEntryType,
		Path:          "https://github.com/nuclio/my-func",
		CodeEntryAttributes: map[string]interface{}{
			"branch":  "master",
			"workDir": "/my-python-func",
		},
	}
	suite.testResolveFunctionPathArchive(buildConfiguration, archiveFileURL)
}

func (suite *testSuite) TestResolveFunctionPathS3CodeEntry() {

	// validate values passed to the mocked function
	suite.mockS3Client.
		On("Download",
			mock.Anything,
			mock.MatchedBy(common.GenerateStringMatchVerifier("my-s3-bucket")),
			mock.MatchedBy(common.GenerateStringMatchVerifier("funcs.zip")),
			mock.MatchedBy(common.GenerateStringMatchVerifier("my-s3-region")),
			mock.MatchedBy(common.GenerateStringMatchVerifier("my-s3-access-key-id")),
			mock.MatchedBy(common.GenerateStringMatchVerifier("my-s3-secret-access-key")),
			mock.MatchedBy(common.GenerateStringMatchVerifier("my-s3-session-token"))).
		Return(nil).
		Once()

	buildConfiguration := functionconfig.Build{
		CodeEntryType: S3EntryType,
		Path:          "",
		CodeEntryAttributes: map[string]interface{}{
			"s3Bucket":          "my-s3-bucket",
			"s3ItemKey":         "funcs.zip",
			"s3Region":          "my-s3-region",
			"s3AccessKeyId":     "my-s3-access-key-id",
			"s3SecretAccessKey": "my-s3-secret-access-key",
			"s3SessionToken":    "my-s3-session-token",
			"workDir":           "/funcs/my-python-func",
		},
	}
	suite.testResolveFunctionPathArchive(buildConfiguration, "")
}

func (suite *testSuite) TestResolveFunctionPathGitCodeEntry() {
	for _, testCase := range []struct {
		Name               string
		BuildConfiguration functionconfig.Build
	}{

		// Github
		{
			Name: "GithubBranch",
			BuildConfiguration: functionconfig.Build{
				CodeEntryType: GitEntryType,
				Path:          "https://github.com/sahare92/test-nuclio-cet.git",
				CodeEntryAttributes: map[string]interface{}{
					"workDir": "go-function",
					"branch":  "go-func",
				},
			},
		},
		{
			Name: "GithubTag",
			BuildConfiguration: functionconfig.Build{
				CodeEntryType: GitEntryType,
				Path:          "https://github.com/sahare92/test-nuclio-cet.git",
				CodeEntryAttributes: map[string]interface{}{
					"workDir": "go-function",
					"tag":     "0.0.1",
				},
			},
		},
		{
			Name: "GithubReference",
			BuildConfiguration: functionconfig.Build{
				CodeEntryType: GitEntryType,
				Path:          "https://github.com/sahare92/test-nuclio-cet.git",
				CodeEntryAttributes: map[string]interface{}{
					"workDir":   "go-function",
					"reference": "refs/heads/go-func",
				},
			},
		},

		// BitBucket
		{
			Name: "BitBucketBranch",
			BuildConfiguration: functionconfig.Build{
				CodeEntryType: GitEntryType,
				Path:          "https://bitbucket.org/saharel/test-nuclio-cet.git",
				CodeEntryAttributes: map[string]interface{}{
					"workDir": "go-function",
					"branch":  "go-func",
				},
			},
		},
		{
			Name: "BitBucketTag",
			BuildConfiguration: functionconfig.Build{
				CodeEntryType: GitEntryType,
				Path:          "https://bitbucket.org/saharel/test-nuclio-cet.git",
				CodeEntryAttributes: map[string]interface{}{
					"workDir": "go-function",
					"tag":     "0.0.1",
				},
			},
		},

		// Azure Devops
		{
			Name: "AzureDevopsBranch",
			BuildConfiguration: functionconfig.Build{
				CodeEntryType: GitEntryType,
				Path:          "https://dev.azure.com/sahar920089/test-nuclio-cet/_git/test-nuclio-cet",
				CodeEntryAttributes: map[string]interface{}{
					"workDir": "go-function",
					"branch":  "go-func",
				},
			},
		},
		{
			Name: "AzureDevopsTag",
			BuildConfiguration: functionconfig.Build{
				CodeEntryType: GitEntryType,
				Path:          "https://dev.azure.com/sahar920089/test-nuclio-cet/_git/test-nuclio-cet",
				CodeEntryAttributes: map[string]interface{}{
					"workDir": "go-function",
					"tag":     "0.0.1",
				},
			},
		},
	} {
		suite.Run(testCase.Name, func() {
			err := suite.builder.createTempDir()
			suite.Require().NoError(err)

			suite.builder.options.FunctionConfig.Spec.Build = testCase.BuildConfiguration

			path, _, err := suite.builder.resolveFunctionPath(testCase.BuildConfiguration.Path)
			suite.Require().NoError(err)

			// make sure the path is set to the work dir inside the downloaded folder
			destinationWorkDir := filepath.Join("/download", testCase.BuildConfiguration.CodeEntryAttributes["workDir"].(string))
			suite.Require().Equal(suite.builder.tempDir+destinationWorkDir, path)

			// get git reference as it was planted on the code inside the remote git repository
			gitAttributes, err := suite.builder.parseGitAttributes()
			suite.Require().NoError(err)
			referenceName, err := gitcommon.ResolveReference(suite.builder.options.FunctionConfig.Spec.Build.Path, gitAttributes)
			suite.Require().NoError(err)

			// make sure our test file was downloaded correctly
			handlerFileContents, err := ioutil.ReadFile(filepath.Join(path, "/main.go"))
			suite.Require().Equal(fmt.Sprintf(`package main

import (
    "github.com/nuclio/nuclio-sdk-go"
)

func Handler(context *nuclio.Context, event nuclio.Event) (interface{}, error) {
    return "Hello from reference: %s", nil
}

`, referenceName), string(handlerFileContents))
			suite.Require().NoError(err)

			suite.builder.cleanupTempDir() // nolint: errcheck
		})
	}
}

// test that when `spec.build.image` is given, it is enriched correctly
func (suite *testSuite) TestImageNameConfigurationEnrichment() {
	suite.builder.options.FunctionConfig.Meta.Name = "name"
	suite.builder.options.FunctionConfig.Spec.Handler = "handler"
	suite.builder.options.FunctionConfig.Spec.Runtime = "python3.6"

	type testAttributes struct {
		inputImageName             string
		expectedProcessorImageName string
		expectedProcessorImageTag  string
	}

	// test different possibilities of image names
	for _, testAttributesInstance := range []testAttributes{
		{
			"imagename",
			"imagename",
			"latest",
		},
		{
			"imagename:imagetag",
			"imagename",
			"imagetag",
		},
		{
			"username.x.com/imagename",
			"username.x.com/imagename",
			"latest",
		},
		{
			"username.x.com/imagename:imagetag",
			"username.x.com/imagename",
			"imagetag",
		},
		{
			"x.com/<some-user>/imagename",
			"x.com/<some-user>/imagename",
			"latest",
		},
		{
			"x.com/<some-user>/imagename:imagetag",
			"x.com/<some-user>/imagename",
			"imagetag",
		},
	} {

		suite.builder.options.FunctionConfig.Spec.Build.Image = testAttributesInstance.inputImageName
		err := suite.builder.validateAndEnrichConfiguration()
		suite.Assert().NoError(err)
		suite.Assert().Equal(testAttributesInstance.expectedProcessorImageName, suite.builder.processorImage.imageName)
		suite.Assert().Equal(testAttributesInstance.expectedProcessorImageTag, suite.builder.processorImage.imageTag)

		// cleanup for the next test
		suite.builder.processorImage.imageName = ""
		suite.builder.processorImage.imageTag = ""
	}
}

func (suite *testSuite) testResolveFunctionPathRemoteCodeFile(fileExtension string) {

	// mock http response
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	codeFileContent := "some code..."
	responder := func(req *http.Request) (*http.Response, error) {
		responder := httpmock.NewStringResponder(200, codeFileContent)
		response, err := responder(req)
		if err != nil {
			return nil, errors.Wrap(err, "Could not get response")
		}
		response.ContentLength = int64(len(codeFileContent))
		return response, err
	}
	codeFileURL := "http://some-address.com/my-func." + fileExtension
	httpmock.RegisterResponder("GET", codeFileURL, responder)

	// the code file will be "downloaded" here
	err := suite.builder.createTempDir()
	suite.Require().NoError(err)

	defer suite.builder.cleanupTempDir() // nolint: errcheck

	path, _, err := suite.builder.resolveFunctionPath(codeFileURL)
	suite.Require().NoError(err)

	expectedFilePath := filepath.Join(suite.builder.tempDir, "/download/my-func."+fileExtension)
	suite.Equal(expectedFilePath, path)

	resultSourceCode, err := ioutil.ReadFile(expectedFilePath)
	suite.Assert().NoError(err)

	suite.Assert().Equal(codeFileContent, string(resultSourceCode))
}

func (suite *testSuite) mockArchiveFileURLEndpoint(buildConfiguration functionconfig.Build, archiveFileURL string) {
	if buildConfiguration.CodeEntryType != S3EntryType {
		httpmock.Activate()
		functionArchiveFileBytes, err := ioutil.ReadFile(FunctionsArchiveFilePath)

		responder := func(req *http.Request) (*http.Response, error) {
			response := &http.Response{
				Status:        "200",
				StatusCode:    200,
				Body:          httpmock.NewRespBodyFromBytes(functionArchiveFileBytes),
				Header:        http.Header{},
				ContentLength: int64(len(functionArchiveFileBytes)),
			}
			return response, err
		}
		httpmock.RegisterResponder("GET", archiveFileURL, responder)
	}
}

func (suite *testSuite) testResolveFunctionPathArchive(buildConfiguration functionconfig.Build, archiveFileURL string) {
	var destinationWorkDir string

	suite.mockArchiveFileURLEndpoint(buildConfiguration, archiveFileURL)

	err := suite.builder.createTempDir()
	suite.Require().NoError(err)

	suite.builder.options.FunctionConfig.Spec.Build = buildConfiguration

	path, _, err := suite.builder.resolveFunctionPath(buildConfiguration.Path)
	suite.Require().NoError(err)

	// make sure the path is set to the work dir inside the decompressed folder
	if buildConfiguration.CodeEntryType == GithubEntryType {
		destinationWorkDir = filepath.Join("/funcs", buildConfiguration.CodeEntryAttributes["workDir"].(string))
	} else {
		destinationWorkDir = buildConfiguration.CodeEntryAttributes["workDir"].(string)
	}
	suite.Equal(suite.builder.tempDir+"/decompress"+destinationWorkDir, path)

	// make sure our test python file is inside the decompress folder
	decompressedPythonFileContent, err := ioutil.ReadFile(filepath.Join(path, "/main.py"))
	suite.Equal(`def handler(context, event):
	return "hello world"
`, string(decompressedPythonFileContent))
	suite.Require().NoError(err)

	suite.builder.cleanupTempDir() // nolint: errcheck
	httpmock.DeactivateAndReset()
}

func (suite *testSuite) testResolveFunctionPathArchiveBadWorkDir(buildConfiguration functionconfig.Build,
	archiveFileURL string,
	expectedError string) {

	suite.mockArchiveFileURLEndpoint(buildConfiguration, archiveFileURL)

	err := suite.builder.createTempDir()
	suite.Require().NoError(err)

	suite.builder.options.FunctionConfig.Spec.Build = buildConfiguration

	_, _, err = suite.builder.resolveFunctionPath(buildConfiguration.Path)
	suite.EqualError(errors.RootCause(err), expectedError)

	suite.builder.cleanupTempDir() // nolint: errcheck
	httpmock.DeactivateAndReset()
}

func TestBuilderSuite(t *testing.T) {
	if testing.Short() {
		return
	}

	suite.Run(t, new(testSuite))
}
