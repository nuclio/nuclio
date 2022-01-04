//go:build test_integration && test_local

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

package workerwait

import (
	"path"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/platform"
	"github.com/nuclio/nuclio/pkg/processor/trigger/http/test/suite"

	"github.com/stretchr/testify/suite"
)

type workerWaitTestSuite struct { // nolint
	httpsuite.TestSuite
}

func (suite *workerWaitTestSuite) TestGolang() {
	suite.deploySleeperWithTimeout(0, 5, 1, 4)
	suite.deploySleeperWithTimeout(10000, 5, 5, 0)
}

func (suite *workerWaitTestSuite) deploySleeperWithTimeout(workerAvailabilityTimeoutMilliseconds int,
	numConcurrentRequests int,
	expectedSuccesses int,
	expectedErrors int) {

	createFunctionOptions := suite.GetDeployOptions("sleeper",
		path.Join(suite.GetTestFunctionsDir(), "common", "sleeper", "golang"))

	createFunctionOptions.FunctionConfig.Spec.Runtime = "golang"
	createFunctionOptions.FunctionConfig.Spec.Triggers = map[string]functionconfig.Trigger{
		"http": {
			Kind:                                  "http",
			MaxWorkers:                            1,
			WorkerAvailabilityTimeoutMilliseconds: &workerAvailabilityTimeoutMilliseconds,
		},
	}

	suite.DeployFunction(createFunctionOptions, func(deployResult *platform.CreateFunctionResult) bool {
		var successes, errors uint64

		waitGroup := sync.WaitGroup{}
		waitGroup.Add(numConcurrentRequests)

		for idx := 0; idx < numConcurrentRequests; idx++ {
			go func() {
				testRequest := httpsuite.Request{
					RequestMethod: "POST",
					RequestPort:   deployResult.Port,
					ExpectedResponseBody: func(body []byte, statusCode int) {
						switch statusCode {
						case 200:
							suite.Logger.DebugWith("Got 200")
							atomic.AddUint64(&successes, 1)
						case 503:
							suite.Logger.DebugWith("Got 503")
							atomic.AddUint64(&errors, 1)
						default:
							suite.FailNow("Unexpected response")
						}
					},
				}

				suite.SendRequestVerifyResponse(&testRequest)

				waitGroup.Done()
			}()
		}

		// wait until all 4 requests are done
		waitGroup.Wait()

		suite.Require().Equal(int(successes), expectedSuccesses)
		suite.Require().Equal(int(errors), expectedErrors)

		return true
	})
}

func TestWorkerWaitSuite(t *testing.T) {
	if testing.Short() {
		return
	}

	suite.Run(t, new(workerWaitTestSuite))
}
