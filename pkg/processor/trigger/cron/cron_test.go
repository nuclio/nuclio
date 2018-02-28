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

package cron

import (
	"testing"
	"time"

	"github.com/nuclio/nuclio/pkg/errors"
	"github.com/nuclio/nuclio/pkg/processor/test/suite"

	cronlib "github.com/robfig/cron"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

type TestSuite struct {
	processorsuite.TestSuite
	trigger cron
}

func (suite *TestSuite) SetupSuite() {
	suite.TestSuite.SetupSuite()
}

func (suite *TestSuite) TearDownSuite() {
	suite.TestSuite.TearDownTest()
}

func (suite *TestSuite) SetupTest() {
	suite.trigger = cron{}
	suite.trigger.Logger = suite.Logger.GetChild("cron")
}

func (suite *TestSuite) TestGetMissedTicksIntervalHandlesNoMisses() {
	var err error
	suite.trigger.schedule, err = suite.getInterval("5s")
	suite.Assert().NoError(err, "Invalid interval string")

	lastRuntime := time.Now()
	missedTicks := suite.trigger.getMissedTicks(suite.trigger.schedule, lastRuntime)

	suite.Assert().EqualValues(0, missedTicks)
}

func (suite *TestSuite) TestGetMissedTicksScheduleHandlesNoMisses() {
	var err error
	suite.trigger.schedule, err = suite.getSchedule("*/5 * * * *")
	suite.Assert().NoError(err, "Invalid interval string")

	lastRuntime := time.Now()
	missedTicks := suite.trigger.getMissedTicks(suite.trigger.schedule, lastRuntime)

	suite.Assert().EqualValues(0, missedTicks)
}

func (suite *TestSuite) TestGetMissedTicksIntervalCountsMisses() {
	var err error
	suite.trigger.schedule, err = suite.getInterval("5s")
	suite.Assert().NoError(err, "Invalid interval string")

	lastTimeDifference, err := time.ParseDuration("10s")
	suite.Require().NoError(err)

	lastRuntime := time.Now().Add(-lastTimeDifference)
	missedTicks := suite.trigger.getMissedTicks(suite.trigger.schedule, lastRuntime)

	suite.Assert().EqualValues(2, missedTicks)
}

func (suite *TestSuite) TestGetMissedTicksScheduleCountsMisses() {
	var err error
	suite.trigger.schedule, err = suite.getSchedule("*/5 * * * *")
	suite.Assert().NoError(err, "Invalid interval string")

	lastTimeDifference, err := time.ParseDuration("10s")
	suite.Require().NoError(err)

	lastRuntime := time.Now().Add(-lastTimeDifference)
	missedTicks := suite.trigger.getMissedTicks(suite.trigger.schedule, lastRuntime)

	suite.Assert().EqualValues(2, missedTicks)
}

func (suite *TestSuite) TestGetNextEventSubmitDelayIntervalNoMisses() {
	var err error

	suite.trigger.schedule, err = suite.getInterval("5s")
	suite.Assert().NoError(err, "Invalid interval string")

	lastRuntime := time.Now()
	nextEventDelay := suite.trigger.getNextEventSubmitDelay(suite.trigger.schedule, lastRuntime)

	expectedEventDelay, err := time.ParseDuration("5s")
	suite.Assert().NoError(err, "Invalid interval string")

	suite.Assert().Condition(
		assert.Comparison(func() bool { return nextEventDelay > 0 && nextEventDelay < expectedEventDelay }),
		"Expected delay between 0 and %s",
		expectedEventDelay,
		nextEventDelay,
	)
}

func (suite *TestSuite) TestGetNextEventSubmitDelayScheduleNoMisses() {
	var err error

	suite.trigger.schedule, err = suite.getSchedule("*/5 * * * *")
	suite.Assert().NoError(err, "Invalid interval string")

	lastRuntime := time.Now()
	nextEventDelay := suite.trigger.getNextEventSubmitDelay(suite.trigger.schedule, lastRuntime)

	expectedEventDelay, err := time.ParseDuration("5s")
	suite.Assert().NoError(err, "Invalid interval string")

	suite.Assert().Condition(
		assert.Comparison(func() bool { return nextEventDelay > 0 && nextEventDelay < expectedEventDelay }),
		"Expected delay between 0 and %s",
		expectedEventDelay,
		nextEventDelay,
	)
}

func (suite *TestSuite) TestGetNextEventSubmitDelayIntervalRunsImmediatelyOnMiss() {
	var err error

	suite.trigger.schedule, err = suite.getInterval("5s")
	suite.Assert().NoError(err, "Invalid interval string")

	lastTimeDifference, err := time.ParseDuration("10s")
	suite.Require().NoError(err)

	lastRuntime := time.Now().Add(-lastTimeDifference)
	nextEventDelay := suite.trigger.getNextEventSubmitDelay(suite.trigger.schedule, lastRuntime)

	suite.Assert().EqualValues(0, nextEventDelay)
}

func (suite *TestSuite) TestGetNextEventSubmitDelayScheduleRunsImmediatelyOnMiss() {
	var err error

	suite.trigger.schedule, err = suite.getSchedule("*/5 * * * *")
	suite.Assert().NoError(err, "Invalid interval string")

	lastTimeDifference, err := time.ParseDuration("10s")
	suite.Require().NoError(err)

	lastRuntime := time.Now().Add(-lastTimeDifference)
	nextEventDelay := suite.trigger.getNextEventSubmitDelay(suite.trigger.schedule, lastRuntime)

	suite.Assert().EqualValues(0, nextEventDelay)
}

func (suite *TestSuite) getInterval(delay string) (cronlib.Schedule, error) {
	delayDuration, err := time.ParseDuration(delay)
	if err != nil {
		return nil, errors.Wrapf(err, "Failed to parse duration string %s", delay)
	}

	return cronlib.ConstantDelaySchedule{Delay: delayDuration}, nil
}

func (suite *TestSuite) getSchedule(crontab string) (cronlib.Schedule, error) {
	schedule, err := cronlib.Parse(crontab)
	if err != nil {
		return nil, errors.Wrapf(err, "Failed to parse cron string %s", crontab)
	}

	return schedule, nil
}

func TestCronSuite(t *testing.T) {
	suite.Run(t, new(TestSuite))
}
