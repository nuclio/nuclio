//go:build test_integration && test_local && test_benchmark

package test

import (
	"fmt"
	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/platform"
	httpsuite "github.com/nuclio/nuclio/pkg/processor/trigger/http/test/suite"
	"github.com/stretchr/testify/suite"
	"net/http"
	"os"
	"path"
	"testing"
	"time"
)

type BenchmarkTestSuite struct {
	httpsuite.TestSuite
	runtime    string
	reportFile *os.File
}

func (suite *BenchmarkTestSuite) SetupTest() {
	suite.TestSuite.SetupTest()

	suite.Runtime = suite.runtime
	suite.RuntimeDir = "python"
	suite.FunctionDir = path.Join(suite.GetNuclioSourceDir(), "pkg", "processor", "runtime", "python", "test")

	var err error
	suite.reportFile, err = os.OpenFile("benchmark_report.txt", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	suite.Require().NoError(err)
}

func (suite *BenchmarkTestSuite) TearDownSuite() {
	if suite.reportFile != nil {
		_ = suite.reportFile.Close()
	}
}

func (suite *BenchmarkTestSuite) TestSleepBenchmark() {
	statusOK := http.StatusOK

	// Test cases with modes, total requests, and parallelism levels
	for _, testCase := range []struct {
		name               string
		workMode           functionconfig.TriggerWorkMode
		numRequests        int
		requestsInParallel int
	}{
		{"sync_sequential",
			functionconfig.SyncTriggerWorkMode,
			1000,
			1,
		},
		{"sync_50_parallel",
			functionconfig.SyncTriggerWorkMode,
			1000,
			50,
		},
		{"async_sequential",
			functionconfig.AsyncTriggerWorkMode,
			1000,
			1,
		},
		{
			"async_50_parallel",
			functionconfig.AsyncTriggerWorkMode,
			1000,
			50,
		},
	} {
		suite.Run(testCase.name, func() {
			createFunctionOptions := suite.getDeployOptions("asyncer",
				suite.GetFunctionPath("outputter"),
				testCase.workMode)

			createFunctionOptions.FunctionConfig.Spec.Handler = "async_outputter:handler"
			createFunctionOptions.FunctionConfig.Spec.Build.Commands = []string{
				"python -m pip install aiofile==3.5.0",
			}

			request := &httpsuite.Request{
				Name:                       "async sleep",
				RequestBody:                "sleep10ms",
				ExpectedResponseBody:       "slept",
				ExpectedResponseStatusCode: &statusOK,
			}

			_, duration := suite.DeployFunctionAndRequestsWithBenchmark(
				createFunctionOptions,
				request,
				testCase.numRequests,
				testCase.requestsInParallel,
			)

			suite.writeBenchmarkResult(suite.T().Name(), string(testCase.workMode),
				duration, testCase.numRequests, testCase.requestsInParallel)
		})
	}
}

func (suite *BenchmarkTestSuite) writeBenchmarkResult(testName, mode string, duration time.Duration, numRequests int, requestsInParallel int) {
	avg := duration / time.Duration(numRequests)

	fmt.Fprintf(suite.reportFile,
		"[Test: %s] [Runtime: %s] [Mode: %s] [TotalRequests: %d] [Parallelism: %d] took %s (avg %s)\n",
		testName,
		suite.runtime,
		mode,
		numRequests,
		requestsInParallel,
		duration,
		avg,
	)
}

func (suite *BenchmarkTestSuite) getDeployOptions(functionName, functionPath string, mode functionconfig.TriggerWorkMode) *platform.CreateFunctionOptions {
	if mode == functionconfig.AsyncTriggerWorkMode {
		return suite.GetDeployOptionsAsync(functionName, functionPath, 1)
	}
	return suite.GetDeployOptions(functionName, functionPath)
}

func TestBenchmarkTestSuite(t *testing.T) {
	if testing.Short() {
		return
	}

	for _, testCase := range []struct {
		runtimeName string
	}{
		{runtimeName: "python:3.9"},
		{runtimeName: "python:3.10"},
		{runtimeName: "python:3.11"},
		{runtimeName: "python:3.12"},
	} {
		t.Run(testCase.runtimeName, func(t *testing.T) {
			testSuite := new(BenchmarkTestSuite)
			testSuite.runtime = testCase.runtimeName
			suite.Run(t, testSuite)
		})
	}
}
