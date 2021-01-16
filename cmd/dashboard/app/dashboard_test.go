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

package app

import (
	"testing"
	"time"

	"github.com/nuclio/nuclio/pkg/common"
	"github.com/nuclio/nuclio/pkg/common/statusprovider"
	"github.com/nuclio/nuclio/pkg/dockerclient"

	"github.com/nuclio/errors"
	"github.com/nuclio/logger"
	nucliozap "github.com/nuclio/zap"
	"github.com/stretchr/testify/suite"
)

type DashboardTestSuite struct {
	suite.Suite
	logger    logger.Logger
	dashboard *Dashboard
}

func (suite *DashboardTestSuite) SetupSuite() {
	suite.logger, _ = nucliozap.NewNuclioZapTest("test")
}

func (suite *DashboardTestSuite) SetupTest() {
	suite.dashboard = &Dashboard{
		logger: suite.logger,
		status: statusprovider.Initializing,
	}
}

func (suite *DashboardTestSuite) TestDashboardStatusFailed() {
	stopChan := make(chan struct{})
	defer close(stopChan)
	maxConsecutiveErrors := 5
	interval := 100 * time.Millisecond
	dockerClient := dockerclient.NewMockDockerClient()
	dockerClient.
		On("GetVersion").
		Return("", errors.New("Something bad happened")).
		Times(maxConsecutiveErrors)

	// run in the background
	go suite.dashboard.MonitorDockerConnectivity(interval,
		maxConsecutiveErrors,
		dockerClient,
		stopChan)

	err := common.RetryUntilSuccessful(3*time.Second,
		interval,
		func() bool {
			return suite.dashboard.GetStatus().OneOf(statusprovider.Error)
		})
	suite.Require().NoError(err, "Exhausted waiting for dashboard status to change")
	dockerClient.AssertExpectations(suite.T())
}

func (suite *DashboardTestSuite) TestNoMonitorWhenDashboardStatusFailed() {
	stopChan := make(chan struct{})
	defer close(stopChan)
	interval := 50 * time.Millisecond
	suite.dashboard.SetStatus(statusprovider.Error)
	dockerClient := dockerclient.NewMockDockerClient()

	// run in the background
	go suite.dashboard.MonitorDockerConnectivity(interval,
		5,
		dockerClient,
		stopChan)

	// sleep for a bit
	time.Sleep(time.Duration(10) * interval)

	// shutdown monitor
	stopChan <- struct{}{}

	dockerClient.AssertNotCalled(suite.T(), "GetVersion")
	dockerClient.AssertExpectations(suite.T())
}

func (suite *DashboardTestSuite) TestStayReadyOnTransientFailures() {
	stopChan := make(chan struct{})
	defer close(stopChan)
	maxConsecutiveErrors := 3
	interval := 100 * time.Millisecond
	dockerClient := dockerclient.NewMockDockerClient()

	// return OK, error, error, OK, ...
	dockerClient.
		On("GetVersion").
		Return("1", nil).
		Once()
	dockerClient.
		On("GetVersion").
		Return("", errors.New("Something bad happened")).
		Twice()
	dockerClient.
		On("GetVersion").
		Return("2", nil).
		Once()

	// run in the background
	go suite.dashboard.MonitorDockerConnectivity(interval,
		maxConsecutiveErrors,
		dockerClient,
		stopChan)

	err := common.RetryUntilSuccessful(3*time.Second,
		interval,
		func() bool {
			return len(dockerClient.Calls) >= 4
		})
	suite.Require().NoError(err, "Exhausted waiting for docker client to perform healthcheck validation")
	dockerClient.AssertExpectations(suite.T())

}

func TestDashboardTestSuite(t *testing.T) {
	suite.Run(t, new(DashboardTestSuite))
}
