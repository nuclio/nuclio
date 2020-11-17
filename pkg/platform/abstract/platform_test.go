package abstract

import (
	"bufio"
	"io/ioutil"
	"os"
	"path"
	"testing"

	"github.com/nuclio/nuclio/pkg/common"
	"github.com/nuclio/nuclio/pkg/containerimagebuilderpusher"
	"github.com/nuclio/nuclio/pkg/dockerclient"
	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/platform"
	"github.com/nuclio/nuclio/pkg/platformconfig"

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
	TempDir          string
	CleanupTemp      bool
	DefaultNamespace string
}

func (suite *TestAbstractSuite) SetupSuite() {
	var err error

	common.SetVersionFromEnv()

	suite.DefaultNamespace = "nuclio"

	suite.Logger, err = nucliozap.NewNuclioZapTest("test")
	suite.Require().NoError(err, "Logger should create successfully")

	testPlatform := &TestPlatform{
		logger:         suite.Logger,
		suiteAssertion: suite.Assert(),
	}
	suite.Platform, err = NewPlatform(suite.Logger, testPlatform, &platformconfig.Config{})
	suite.Require().NoError(err, "Could not create platform")

	suite.Platform.ContainerBuilder, err = containerimagebuilderpusher.NewNop(suite.Logger, nil)
	suite.Require().NoError(err)
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
		functionName := string(rune(idx + 65))
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
		suite.Require().NoError(err, "Failed to enrich function")

		err = suite.Platform.ValidateCreateFunctionOptions(createFunctionOptions)
		if MinMaxReplicas.shouldFailValidation {
			suite.Error(err, "Validation should fail")
			suite.Logger.DebugWith("Validation failed as expected ", "functionName", functionName)
			continue
		}
		suite.Require().NoError(err, "Failed to validate function")
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

func (suite *TestAbstractSuite) TestFunctionTriggersEnriched() {
	for idx, testCase := range []struct {
		triggers                 map[string]functionconfig.Trigger
		expectedEnrichedTriggers map[string]functionconfig.Trigger
		shouldFailValidation     bool
	}{

		// enrich maxWorkers to 1
		{
			triggers: map[string]functionconfig.Trigger{
				"some-trigger": {
					Kind: "http",
				},
			},
			expectedEnrichedTriggers: map[string]functionconfig.Trigger{
				"some-trigger": {
					Kind:       "http",
					MaxWorkers: 1,
				},
			},
		},

		// enrich with default http trigger
		{
			triggers: nil,
			expectedEnrichedTriggers: func() map[string]functionconfig.Trigger {
				defaultHTTPTrigger := functionconfig.GetDefaultHTTPTrigger()
				return map[string]functionconfig.Trigger{
					defaultHTTPTrigger.Name: defaultHTTPTrigger,
				}
			}(),
		},

		// do not allow more than 1 http trigger
		{
			triggers: map[string]functionconfig.Trigger{
				"firstHTTPTrigger": {
					Kind: "http",
				},
				"secondHTTPTrigger": {
					Kind: "http",
				},
			},
			shouldFailValidation: true,
		},
	} {
		// name it with index and shift with 65 to get A as first letter
		functionName := string(rune(idx + 65))
		functionConfig := *functionconfig.NewConfig()

		createFunctionOptions := &platform.CreateFunctionOptions{
			Logger:         suite.Logger,
			FunctionConfig: functionConfig,
		}
		createFunctionOptions.FunctionConfig.Meta.Name = functionName
		createFunctionOptions.FunctionConfig.Meta.Labels = map[string]string{
			"nuclio.io/project-name": platform.DefaultProjectName,
		}
		createFunctionOptions.FunctionConfig.Spec.Triggers = testCase.triggers
		suite.Logger.DebugWith("Checking function ", "functionName", functionName)

		err := suite.Platform.EnrichCreateFunctionOptions(createFunctionOptions)
		suite.Require().NoError(err, "Failed to enrich function")

		err = suite.Platform.ValidateCreateFunctionOptions(createFunctionOptions)
		if testCase.shouldFailValidation {
			suite.Require().Error(err, "Validation passed unexpectedly")
			continue
		}

		suite.Require().NoError(err, "Validation failed unexpectedly")
		suite.Equal(testCase.expectedEnrichedTriggers,
			createFunctionOptions.FunctionConfig.Spec.Triggers)
	}
}

func (suite *TestAbstractSuite) TestGetProcessorLogsOnMultiWorker() {
	suite.testGetProcessorLogsTestFromFile(MultiWorkerFunctionLogsFilePath)
}

func (suite *TestAbstractSuite) TestGetProcessorLogsOnPanic() {
	suite.testGetProcessorLogsTestFromFile(PanicFunctionLogsFilePath)
}

func (suite *TestAbstractSuite) TestGetProcessorLogsOnGoWithCallStack() {
	suite.testGetProcessorLogsTestFromFile(GoWithCallStackFunctionLogsFilePath)
}

func (suite *TestAbstractSuite) TestGetProcessorLogsWithSpecialSubstrings() {
	suite.testGetProcessorLogsTestFromFile(SpecialSubstringsFunctionLogsFilePath)
}

func (suite *TestAbstractSuite) TestGetProcessorLogsWithConsecutiveDuplicateMessages() {
	suite.testGetProcessorLogsTestFromFile(ConsecutiveDuplicateFunctionLogsFilePath)
}

// Test that GetProcessorLogs() generates the expected formattedPodLogs and briefErrorsMessage
// Expects 3 files inside functionLogsFilePath: (kept in these constants)
// - FunctionLogsFile
// - FormattedFunctionLogsFile
// - BriefErrorsMessageFile
func (suite *TestAbstractSuite) testGetProcessorLogsTestFromFile(functionLogsFilePath string) {
	functionLogsFile, err := os.Open(path.Join(functionLogsFilePath, FunctionLogsFile))
	suite.Require().NoError(err, "Failed to read function logs file")

	functionLogsScanner := bufio.NewScanner(functionLogsFile)

	formattedPodLogs, briefErrorsMessage := suite.Platform.GetProcessorLogsAndBriefError(functionLogsScanner)

	expectedFormattedFunctionLogsFileBytes, err := ioutil.ReadFile(path.Join(functionLogsFilePath, FormattedFunctionLogsFile))
	suite.Require().NoError(err, "Failed to read formatted function logs file")
	suite.Assert().Equal(string(expectedFormattedFunctionLogsFileBytes), formattedPodLogs)

	expectedBriefErrorsMessageFileBytes, err := ioutil.ReadFile(path.Join(functionLogsFilePath, BriefErrorsMessageFile))
	suite.Require().NoError(err, "Failed to read brief errors message file")
	suite.Assert().Equal(string(expectedBriefErrorsMessageFileBytes), briefErrorsMessage)
}

func TestAbstractPlatformTestSuite(t *testing.T) {
	suite.Run(t, new(TestAbstractSuite))
}
