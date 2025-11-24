//go:build test_unit

/*
Copyright 2025 The Nuclio Authors.

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

package gatherer

import (
	"testing"

	"github.com/nuclio/nuclio/pkg/processor/statistics"
	"github.com/nuclio/nuclio/pkg/processor/trigger"

	"github.com/nuclio/logger"
	nucliozap "github.com/nuclio/zap"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/suite"
)

type triggerGathererTestSuite struct {
	suite.Suite
	logger          logger.Logger
	metricRegistry  *prometheus.Registry
	mockTrigger     *mockTrigger
	triggerGatherer *triggerGatherer
}

func (suite *triggerGathererTestSuite) SetupTest() {
	var err error

	suite.logger, err = nucliozap.NewNuclioZapTest("trigger-gatherer-test")
	suite.Require().NoError(err)

	suite.metricRegistry = prometheus.NewRegistry()

	suite.mockTrigger = &mockTrigger{
		statistics: &trigger.UnsafeStatistics{},
	}

	suite.triggerGatherer, err = newTriggerGatherer(
		"test-instance",
		suite.logger,
		suite.mockTrigger,
		suite.metricRegistry,
	)
	suite.Require().NoError(err)
}

func (suite *triggerGathererTestSuite) TestPrevMetricDifferentFromCurrent() {
	initialStats := suite.mockTrigger.GetStatistics()

	err := suite.triggerGatherer.Gather()
	suite.Require().NoError(err)

	updatedStats := &trigger.UnsafeStatistics{
		EventsHandledSuccessTotal: 25,
		EventsHandledFailureTotal: 5,
		WorkerAllocatorStatistics: statistics.AllocatorStatistics{
			AllocationCount: 30,
		},
	}
	suite.mockTrigger.statistics = updatedStats

	diffStats := suite.mockTrigger.GetStatistics().DiffFrom(initialStats)
	suite.Require().Equal(uint64(25), diffStats.EventsHandledSuccessTotal)
	suite.Require().Equal(uint64(5), diffStats.EventsHandledFailureTotal)
	suite.Require().Equal(uint64(30), diffStats.WorkerAllocatorStatistics.AllocationCount)

	err = suite.triggerGatherer.Gather()
	suite.Require().NoError(err)

	finalStats := &trigger.UnsafeStatistics{
		EventsHandledSuccessTotal: 30,
		EventsHandledFailureTotal: 6,
		WorkerAllocatorStatistics: statistics.AllocatorStatistics{
			AllocationCount: 36,
		},
	}
	suite.mockTrigger.statistics = finalStats

	finalDiffStats := finalStats.DiffFrom(updatedStats)
	suite.Require().Equal(uint64(5), finalDiffStats.EventsHandledSuccessTotal)
	suite.Require().Equal(uint64(1), finalDiffStats.EventsHandledFailureTotal)
	suite.Require().Equal(uint64(6), finalDiffStats.WorkerAllocatorStatistics.AllocationCount)
}

func TestTriggerGatherer(t *testing.T) {
	suite.Run(t, &triggerGathererTestSuite{})
}
