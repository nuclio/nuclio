//go:build test_unit

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
	"context"
	"testing"
	"time"

	"github.com/nuclio/nuclio/pkg/common"
	"github.com/nuclio/nuclio/pkg/common/status"
	"github.com/nuclio/nuclio/pkg/dockerclient"

	"github.com/nuclio/errors"
	"github.com/nuclio/logger"
	nucliozap "github.com/nuclio/zap"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

type DashboardTestSuite struct {
	suite.Suite
	ctx          context.Context
	logger       logger.Logger
	dashboard    *Dashboard
	dockerClient *dockerclient.MockDockerClient
}

func (suite *DashboardTestSuite) SetupSuite() {
	suite.logger, _ = nucliozap.NewNuclioZapTest("test")
}

func (suite *DashboardTestSuite) SetupTest() {
	suite.ctx = context.Background()
	suite.dashboard = &Dashboard{
		logger: suite.logger,
		status: status.Initializing,
	}
	suite.dockerClient = dockerclient.NewMockDockerClient()
}

func (suite *DashboardTestSuite) TearDownTest() {
	suite.dockerClient.AssertExpectations(suite.T())
}

func (suite *DashboardTestSuite) TestDashboardStatusFailed() {
	maxConsecutiveErrors := 5
	interval := 100 * time.Millisecond
	suite.dockerClient.
		On("GetVersion", true).
		Return("", errors.New("Something bad happened")).
		Times(maxConsecutiveErrors)

	// run in the background
	go suite.dashboard.monitorDockerConnectivity(suite.ctx,
		interval,
		maxConsecutiveErrors,
		suite.dockerClient)

	err := common.RetryUntilSuccessful(3*time.Second,
		interval,
		func() bool {
			return suite.dashboard.GetStatus().OneOf(status.Error)
		})
	suite.Require().NoError(err, "Exhausted waiting for dashboard status to change")
}

func (suite *DashboardTestSuite) TestNoMonitorWhenDashboardStatusFailed() {
	interval := 50 * time.Millisecond
	suite.dashboard.SetStatus(status.Error)

	// run in the background
	go suite.dashboard.monitorDockerConnectivity(suite.ctx,
		interval,
		5,
		suite.dockerClient)

	// wait few intervals, let the routine runs for a while
	time.Sleep(time.Duration(10) * interval)

	suite.dockerClient.AssertNotCalled(suite.T(), "GetVersion", mock.Anything)
}

func (suite *DashboardTestSuite) TestStayReadyOnTransientFailures() {
	ctx, cancel := context.WithCancel(suite.ctx)
	maxConsecutiveErrors := 3
	interval := 100 * time.Millisecond

	// return OK, error, error, OK, OK, OK, ...
	suite.dockerClient.
		On("GetVersion", true).
		Return("1", nil).
		Once()
	suite.dockerClient.
		On("GetVersion", true).
		Return("", errors.New("Something bad happened")).
		Twice()
	suite.dockerClient.
		On("GetVersion", true).
		Return("2", nil)

	go func() {
		err := common.RetryUntilSuccessful(3*time.Second,
			interval,
			func() bool {
				return len(suite.dockerClient.Calls) >= 4
			})
		suite.Require().NoError(err, "Exhausted waiting for docker client to perform healthcheck validation")

		// we got the calls we needed, stop routine
		cancel()
	}()

	// run in foreground
	suite.dashboard.monitorDockerConnectivity(ctx,
		interval,
		maxConsecutiveErrors,
		suite.dockerClient)
}

func TestDashboardTestSuite(t *testing.T) {
	suite.Run(t, new(DashboardTestSuite))
}
