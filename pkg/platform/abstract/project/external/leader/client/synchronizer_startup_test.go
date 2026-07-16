//go:build test_unit

/*
Copyright 2026 The Nuclio Authors.

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

package client

import (
	"testing"
	"time"

	"github.com/nuclio/nuclio/pkg/platform"
	leadermock "github.com/nuclio/nuclio/pkg/platform/abstract/project/external/leader/mock"
	internalmock "github.com/nuclio/nuclio/pkg/platform/abstract/project/mock"

	"github.com/nuclio/errors"
	"github.com/nuclio/logger"
	nucliozap "github.com/nuclio/zap"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

type SynchronizerStartupTestSuite struct {
	suite.Suite

	logger                     logger.Logger
	mockInternalProjectsClient *internalmock.Client
	mockLeaderProjectsClient   *leadermock.Client
}

func (suite *SynchronizerStartupTestSuite) SetupTest() {
	var err error
	suite.logger, err = nucliozap.NewNuclioZapTest("test")
	suite.Require().NoError(err)

	suite.mockInternalProjectsClient = &internalmock.Client{}
	suite.mockLeaderProjectsClient = leadermock.NewClient()
}

// TestStartNoSyncWhenDisabled verifies that when syncOnStartup is false and interval is "0",
// neither the startup sync nor the periodic loop runs.
func (suite *SynchronizerStartupTestSuite) TestStartNoSyncWhenDisabled() {
	synchronizer := suite.newSynchronizer("0", false, []string{"ns1"})

	err := synchronizer.Start()
	suite.Require().NoError(err)

	// give any goroutines a moment to manifest (they should not)
	time.Sleep(100 * time.Millisecond)

	suite.mockLeaderProjectsClient.AssertNotCalled(suite.T(), "GetUpdatedAfter", mock.Anything, mock.Anything)
	suite.mockInternalProjectsClient.AssertNotCalled(suite.T(), "Get", mock.Anything, mock.Anything)
}

// TestStartSyncOnStartupRunsOnce verifies that when syncOnStartup is true and interval is "0",
// syncOnce is called exactly once (for the single namespace) and the periodic loop never fires.
func (suite *SynchronizerStartupTestSuite) TestStartSyncOnStartupRunsOnce() {
	namespace := "ns-startup"
	syncDone := make(chan struct{})

	var nilTime *time.Time
	suite.mockLeaderProjectsClient.
		On("GetUpdatedAfter", mock.Anything, nilTime).
		Return([]platform.Project{}, nil).
		Once()

	suite.mockInternalProjectsClient.
		On("Get", mock.Anything, &platform.GetProjectsOptions{
			Meta: platform.ProjectMeta{Namespace: namespace},
		}).
		Return([]platform.Project{}, nil).
		Run(func(args mock.Arguments) { close(syncDone) }).
		Once()

	synchronizer := suite.newSynchronizer("0", true, []string{namespace})

	err := synchronizer.Start()
	suite.Require().NoError(err)

	suite.waitForChannel(syncDone, "startup sync to call internal projects client")

	suite.mockLeaderProjectsClient.AssertExpectations(suite.T())
	suite.mockInternalProjectsClient.AssertExpectations(suite.T())
}

// TestStartSyncOnStartupCoversAllNamespaces verifies that syncOnce iterates over every
// managed namespace, issuing one internal Get call per namespace.
func (suite *SynchronizerStartupTestSuite) TestStartSyncOnStartupCoversAllNamespaces() {
	namespaces := []string{"ns-a", "ns-b", "ns-c"}
	synced := make(chan string, len(namespaces))

	var nilTime *time.Time
	suite.mockLeaderProjectsClient.
		On("GetUpdatedAfter", mock.Anything, nilTime).
		Return([]platform.Project{}, nil)

	for _, ns := range namespaces {
		suite.mockInternalProjectsClient.
			On("Get", mock.Anything, &platform.GetProjectsOptions{
				Meta: platform.ProjectMeta{Namespace: ns},
			}).
			Return([]platform.Project{}, nil).
			Run(func(args mock.Arguments) {
				opts := args.Get(1).(*platform.GetProjectsOptions)
				synced <- opts.Meta.Namespace
			}).
			Once()
	}

	synchronizer := suite.newSynchronizer("0", true, namespaces)

	err := synchronizer.Start()
	suite.Require().NoError(err)

	var observed []string
	timeout := time.After(3 * time.Second)
	for range namespaces {
		select {
		case ns := <-synced:
			observed = append(observed, ns)
		case <-timeout:
			suite.Fail("Timed out waiting for all namespaces to be synced")
			return
		}
	}

	suite.Require().ElementsMatch(namespaces, observed)
	suite.mockLeaderProjectsClient.AssertExpectations(suite.T())
	suite.mockInternalProjectsClient.AssertExpectations(suite.T())
}

// TestStartSyncOnStartupToleratesLeaderError verifies that a leader error during the startup
// sync is absorbed (only logged) and Start() still returns nil.
func (suite *SynchronizerStartupTestSuite) TestStartSyncOnStartupToleratesLeaderError() {
	namespace := "ns-error"
	syncAttempted := make(chan struct{})

	var nilTime *time.Time
	suite.mockLeaderProjectsClient.
		On("GetUpdatedAfter", mock.Anything, nilTime).
		Return([]platform.Project{}, errors.New("leader unavailable")).
		Run(func(args mock.Arguments) { close(syncAttempted) }).
		Once()

	synchronizer := suite.newSynchronizer("0", true, []string{namespace})

	err := synchronizer.Start()
	suite.Require().NoError(err, "Start() must not propagate startup sync errors")

	suite.waitForChannel(syncAttempted, "startup sync to attempt GetUpdatedAfter")

	suite.mockLeaderProjectsClient.AssertExpectations(suite.T())
	suite.mockInternalProjectsClient.AssertNotCalled(suite.T(), "Get", mock.Anything, mock.Anything)
}

// TestStartSyncOnStartupAndPeriodicLoop verifies that when both syncOnStartup is true and a
// non-zero interval is set, the startup sync fires immediately (before the first periodic tick).
// A large interval is used so the periodic tick never fires within the test window.
func (suite *SynchronizerStartupTestSuite) TestStartSyncOnStartupAndPeriodicLoop() {
	namespace := "ns-both"
	startupFired := make(chan struct{})
	internalGetCalled := make(chan struct{})

	var nilTime *time.Time
	suite.mockLeaderProjectsClient.
		On("GetUpdatedAfter", mock.Anything, nilTime).
		Return([]platform.Project{}, nil).
		Run(func(args mock.Arguments) {
			select {
			case <-startupFired:
			default:
				close(startupFired)
			}
		})

	suite.mockInternalProjectsClient.
		On("Get", mock.Anything, &platform.GetProjectsOptions{
			Meta: platform.ProjectMeta{Namespace: namespace},
		}).
		Return([]platform.Project{}, nil).
		Run(func(args mock.Arguments) {
			select {
			case <-internalGetCalled:
			default:
				close(internalGetCalled)
			}
		})

	// use a 1-hour interval so the periodic loop never actually ticks in this test
	synchronizer := suite.newSynchronizer("1h", true, []string{namespace})

	err := synchronizer.Start()
	suite.Require().NoError(err)

	suite.waitForChannel(startupFired, "startup sync to fire before the first periodic tick")
	suite.waitForChannel(internalGetCalled, "startup sync to call the internal projects client")
	suite.mockInternalProjectsClient.AssertExpectations(suite.T())
}

// TestStartInvalidInterval verifies that Start() returns an error when the interval cannot be parsed.
func (suite *SynchronizerStartupTestSuite) TestStartInvalidInterval() {
	synchronizer := suite.newSynchronizer("not-a-duration", false, []string{"ns1"})

	err := synchronizer.Start()
	suite.Require().Error(err)
	suite.Require().Contains(err.Error(), "Failed to parse synchronization interval")
}

func (suite *SynchronizerStartupTestSuite) newSynchronizer(
	intervalStr string,
	syncOnStartup bool,
	namespaces []string,
) *Synchronizer {
	return &Synchronizer{
		logger:                     suite.logger,
		synchronizationIntervalStr: intervalStr,
		syncOnStartup:              syncOnStartup,
		managedNamespaces:          namespaces,
		leaderClient:               suite.mockLeaderProjectsClient,
		internalProjectsClient:     suite.mockInternalProjectsClient,
	}
}

func (suite *SynchronizerStartupTestSuite) waitForChannel(ch <-chan struct{}, description string) {
	select {
	case <-ch:
	case <-time.After(3 * time.Second):
		suite.Fail("Timed out waiting for " + description)
	}
}

func TestSynchronizerStartupTestSuite(t *testing.T) {
	suite.Run(t, new(SynchronizerStartupTestSuite))
}
