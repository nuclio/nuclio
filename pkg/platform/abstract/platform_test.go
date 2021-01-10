package abstract

import (
	"bufio"
	"io/ioutil"
	"os"
	"path"
	"testing"

	"github.com/nuclio/nuclio/pkg/dockerclient"
	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/platform"
	"github.com/nuclio/nuclio/pkg/version"

	"github.com/nuclio/logger"
	"github.com/nuclio/zap"
	"github.com/rs/xid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

type TestPlatform struct {
	platform.Platform
	logger         logger.Logger
	suiteAssertion *assert.Assertions
}

const (
	MultiWorkerFunctionLogsFilePath          = "test/logs_examples/multi_worker"
	PanicFunctionLogsFilePath                = "test/logs_examples/panic"
	GoWithCallStackFunctionLogsFilePath      = "test/logs_examples/go_with_call_stack"
	SpecialSubstringsFunctionLogsFilePath    = "test/logs_examples/special_substrings"
	ConsecutiveDuplicateFunctionLogsFilePath = "test/logs_examples/consecutive_duplicate"
	FunctionLogsFile                         = "function_logs.txt"
	FormattedFunctionLogsFile                = "formatted_function_logs.txt"
	BriefErrorsMessageFile                   = "brief_errors_message.txt"
)

// GetProjects will list existing projects
func (mp *TestPlatform) GetProjects(getProjectsOptions *platform.GetProjectsOptions) ([]platform.Project, error) {
	project, err := platform.NewAbstractProject(mp.logger, nil, platform.ProjectConfig{})
	mp.suiteAssertion.NoError(err, "Failed to create new abstract project")
	return []platform.Project{
		project,
	}, nil
}

type TestAbstractSuite struct {
	suite.Suite
	Logger           logger.Logger
	DockerClient     dockerclient.Client
	Platform         *Platform
	TestID           string
	Runtime          string
	RuntimeDir       string
	FunctionDir      string
	containerID      string
	TempDir          string
	CleanupTemp      bool
	DefaultNamespace string
}

func (suite *TestAbstractSuite) SetupSuite() {
	err := version.Set(&version.Info{
		GitCommit: "c",
		Label:     "latest",
		Arch:      "amd64",
		OS:        "linux",
	})
	suite.Require().NoError(err, "Failed to set version info")

	suite.DefaultNamespace = "nuclio"

	suite.Logger, err = nucliozap.NewNuclioZapTest("test")
	suite.Require().NoError(err, "Logger should create successfully")

	suite.DockerClient, err = dockerclient.NewShellClient(suite.Logger, nil)
	suite.Require().NoError(err, "Docker client should create successfully")

	testPlatform := &TestPlatform{
		logger:         suite.Logger,
		suiteAssertion: suite.Assert(),
	}
	suite.Platform, err = NewPlatform(suite.Logger, testPlatform, nil)
	suite.NoError(err, "Could not create platform")
}

func (suite *TestAbstractSuite) SetupTest() {
	suite.TestID = xid.New().String()
}

// Test function with invalid min max replicas
func (suite *TestAbstractSuite) TestMinMaxReplicas() {
	zero := 0
	one := 1
	two := 2
	for idx, MinMaxReplicas := range []struct {
		MinReplicas          *int
		MaxReplicas          *int
		ExpectedMinReplicas  *int
		ExpectedMaxReplicas  *int
		shouldFailValidation bool
	}{
		{MinReplicas: nil, MaxReplicas: nil, ExpectedMinReplicas: nil, ExpectedMaxReplicas: nil, shouldFailValidation: false},
		{MinReplicas: nil, MaxReplicas: &zero, shouldFailValidation: true},
		{MinReplicas: nil, MaxReplicas: &one, ExpectedMinReplicas: &one, ExpectedMaxReplicas: &one, shouldFailValidation: false},
		{MinReplicas: nil, MaxReplicas: &two, ExpectedMinReplicas: &two, ExpectedMaxReplicas: &two, shouldFailValidation: false},

		{MinReplicas: &zero, MaxReplicas: nil, shouldFailValidation: true},
		{MinReplicas: &zero, MaxReplicas: &zero, shouldFailValidation: true},
		{MinReplicas: &zero, MaxReplicas: &one, ExpectedMinReplicas: &zero, ExpectedMaxReplicas: &one, shouldFailValidation: false},
		{MinReplicas: &zero, MaxReplicas: &two, ExpectedMinReplicas: &zero, ExpectedMaxReplicas: &two, shouldFailValidation: false},

		{MinReplicas: &one, MaxReplicas: nil, ExpectedMinReplicas: &one, ExpectedMaxReplicas: &one, shouldFailValidation: false},
		{MinReplicas: &one, MaxReplicas: &zero, shouldFailValidation: true},
		{MinReplicas: &one, MaxReplicas: &one, ExpectedMinReplicas: &one, ExpectedMaxReplicas: &one, shouldFailValidation: false},
		{MinReplicas: &one, MaxReplicas: &two, ExpectedMinReplicas: &one, ExpectedMaxReplicas: &two, shouldFailValidation: false},

		{MinReplicas: &two, MaxReplicas: nil, ExpectedMinReplicas: &two, ExpectedMaxReplicas: &two, shouldFailValidation: false},
		{MinReplicas: &two, MaxReplicas: &zero, shouldFailValidation: true},
		{MinReplicas: &two, MaxReplicas: &one, shouldFailValidation: true},
		{MinReplicas: &two, MaxReplicas: &two, ExpectedMinReplicas: &two, ExpectedMaxReplicas: &two, shouldFailValidation: false},
	} {

		// name it with index and shift with 65 to get A as first letter
		functionName := string(idx + 65)
		functionConfig := *functionconfig.NewConfig()

		createFunctionOptions := &platform.CreateFunctionOptions{
			Logger:         suite.Logger,
			FunctionConfig: functionConfig,
		}

		createFunctionOptions.FunctionConfig.Meta.Name = functionName
		createFunctionOptions.FunctionConfig.Meta.Labels = map[string]string{
			"nuclio.io/project-name": platform.DefaultProjectName,
		}
		createFunctionOptions.FunctionConfig.Spec.MinReplicas = MinMaxReplicas.MinReplicas
		createFunctionOptions.FunctionConfig.Spec.MaxReplicas = MinMaxReplicas.MaxReplicas
		suite.Logger.DebugWith("Checking function ", "functionName", functionName)

		err := suite.Platform.EnrichCreateFunctionOptions(createFunctionOptions)
		suite.NoError(err, "Failed to enrich function")

		err = suite.Platform.ValidateCreateFunctionOptions(createFunctionOptions)
		if MinMaxReplicas.shouldFailValidation {
			suite.Error(err, "Validation should fail")
			suite.Logger.DebugWith("Validation failed as expected ", "functionName", functionName)
			continue
		}
		suite.NoError(err, "Failed to validate function")
		functionConfigSpec := createFunctionOptions.FunctionConfig.Spec

		if MinMaxReplicas.ExpectedMinReplicas != nil {
			suite.Assert().Equal(*MinMaxReplicas.ExpectedMinReplicas, *functionConfigSpec.MinReplicas)
		} else {
			suite.Assert().Nil(functionConfigSpec.MinReplicas)
		}

		if MinMaxReplicas.ExpectedMaxReplicas != nil {
			suite.Assert().Equal(*MinMaxReplicas.ExpectedMaxReplicas, *functionConfigSpec.MaxReplicas)
		} else {
			suite.Assert().Nil(functionConfigSpec.MaxReplicas)
		}
		suite.Logger.DebugWith("Validation passed successfully", "functionName", functionName)
	}
}

func (suite *TestAbstractSuite) TestGetProcessorLogsOnMultiWorker() {
	suite.testGetProcessorLogs(MultiWorkerFunctionLogsFilePath)
}

func (suite *TestAbstractSuite) TestGetProcessorLogsOnPanic() {
	suite.testGetProcessorLogs(PanicFunctionLogsFilePath)
}

func (suite *TestAbstractSuite) TestGetProcessorLogsOnGoWithCallStack() {
	suite.testGetProcessorLogs(GoWithCallStackFunctionLogsFilePath)
}

func (suite *TestAbstractSuite) TestGetProcessorLogsWithSpecialSubstrings() {
	suite.testGetProcessorLogs(SpecialSubstringsFunctionLogsFilePath)
}

func (suite *TestAbstractSuite) TestValidateFunctionConfigDockerImagesFields() {

	// we do our docker image test coverage on functionConfig.Spec.Build.Image but other fields, like
	// functionConfig.Spec.Image are going through the same validation
	for _, testCase := range []struct {
		buildImage string
		valid      bool
	}{
		// positive cases
		{"just_image-name", true},
		{"image-name-with:tag-333", true},
		{"repo/image:v1.0.0", true},
		{"123.123.123.123:123/image/tag:v1.0.0", true},
		{"some-domain.com/image/tag", true},
		{"some-domain.com/image/tag:v1.1.1-patch1", true},
		{"image/tag", true},
		{"image", true},
		{"image:v1.1.1-patch", true},
		{"ubuntu@sha256:45b23dee08af5e43a7fea6c4cf9c25ccf269ee113168c19722f87876677c5cb2", true},
		{"iguaziodocker/cloud_demo_functions", true},
		{"ghaanvkoqi-snilhltidtkmncpufnhdmpwngszj-naip_test_img", true},
		{"repo/image_with__two-underscores:v1.0.0", true},
		{"repo/underscored_repo__yes/name.with.dot:v1.0.0", true},
		{"underscored_repo__allowed/with/name.with.dot:v1.0.0", true},

		// negative cases
		{"image/tag:v1.0.0 || nc 127.0.0.1 8000 -e /bin/sh ls", false},
		{"123.123.123.123:123/tag:v1.0.0 | echo something", false},
		{"123.123_123.123:123/tag:v1.0.0", false},
		{"gcr_nope.io:80/repo_w_underscore_is_ok/tag:v1.0.0", false},
		{"repo/image:v1.0.0;xyz&netstat", false},
		{"repo/image:v1.0.0;ls|cp&rm", false},
		{"image\" cp something", false},
		{"image\\\" cp something", false},
	} {

		createFunctionOptions := platform.CreateFunctionOptions{}
		createFunctionOptions.FunctionConfig = *functionconfig.NewConfig()
		createFunctionOptions.FunctionConfig.Spec.Build.Image = testCase.buildImage

		suite.Logger.InfoWith("Running function spec sanitization case",
			"createFunctionOptions", createFunctionOptions,
			"valid", testCase.valid)

		err := suite.Platform.ValidateCreateFunctionOptions(&createFunctionOptions)
		if !testCase.valid {
			suite.Require().Error(err, "Validation passed unexpectedly")
			suite.Logger.InfoWith("Expected error received", "err", err, "functionConfig", functionConfig)
			continue
		}
		suite.Require().NoError(err)
	}
}

func (suite *TestAbstractSuite) TestGetProcessorLogsWithConsecutiveDuplicateMessages() {
	suite.testGetProcessorLogs(ConsecutiveDuplicateFunctionLogsFilePath)
}

// Test that GetProcessorLogs() generates the expected formattedPodLogs and briefErrorsMessage
// Expects 3 files inside functionLogsFilePath: (kept in these constants)
// - FunctionLogsFile
// - FormattedFunctionLogsFile
// - BriefErrorsMessageFile
func (suite *TestAbstractSuite) testGetProcessorLogs(functionLogsFilePath string) {
	functionLogsFile, err := os.Open(path.Join(functionLogsFilePath, FunctionLogsFile))
	suite.NoError(err, "Failed to read function logs file")

	functionLogsScanner := bufio.NewScanner(functionLogsFile)

	formattedPodLogs, briefErrorsMessage := suite.Platform.GetProcessorLogsAndBriefError(functionLogsScanner)

	expectedFormattedFunctionLogsFileBytes, err := ioutil.ReadFile(path.Join(functionLogsFilePath, FormattedFunctionLogsFile))
	suite.NoError(err, "Failed to read formatted function logs file")
	suite.Assert().Equal(string(expectedFormattedFunctionLogsFileBytes), formattedPodLogs)

	expectedBriefErrorsMessageFileBytes, err := ioutil.ReadFile(path.Join(functionLogsFilePath, BriefErrorsMessageFile))
	suite.NoError(err, "Failed to read brief errors message file")
	suite.Assert().Equal(string(expectedBriefErrorsMessageFileBytes), briefErrorsMessage)
}

func TestAbstractPlatformTestSuite(t *testing.T) {
	if testing.Short() {
		return
	}

	suite.Run(t, new(TestAbstractSuite))
}
