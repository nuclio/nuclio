package idle_test

import (
	"net/http"
	"testing"
	"time"

	"github.com/nuclio/nuclio/pkg/nexus/common/models/config"
	common "github.com/nuclio/nuclio/pkg/nexus/common/queue"
	scheduler "github.com/nuclio/nuclio/pkg/nexus/common/scheduler"
	idle "github.com/nuclio/nuclio/pkg/nexus/idle/scheduler"
	utils "github.com/nuclio/nuclio/pkg/nexus/utils"
	"github.com/stretchr/testify/suite"
)

type IdleSchedulerTestSuite struct {
	suite.Suite
	is idle.IdleScheduler
}

func (suite *IdleSchedulerTestSuite) SetupTest() {
	sleepDuration := 10 * time.Millisecond

	defaultQueue := common.
		Initialize()
	baseSchedulerConfig := config.BaseNexusSchedulerConfig{
		SleepDuration: sleepDuration,
	}
	nexusConfig := config.
		NewDefaultNexusConfig()

	Client := &http.Client{
		Transport: &utils.MockRoundTripper{},
	}

	baseScheduler := scheduler.
		NewBaseNexusScheduler(defaultQueue, &baseSchedulerConfig, &nexusConfig, Client, nil, nil)

	suite.is = *idle.NewScheduler(baseScheduler)
}

func (suite *IdleSchedulerTestSuite) TestIdleScheduler() {
	taskNames := []string{
		"task1",
		"task2",
		"task3",
		"task4",
		"task5",
		"task6",
		"task7",
		"task8",
		"task9",
		"task10",
	}
	offset := 10
	utils.PushMockedTasksToQueue(&suite.is.BaseNexusScheduler, taskNames, offset)

	suite.is.MaxParallelRequests.Store(1)

	go suite.is.Start()

	time.Sleep(5 * time.Millisecond)

	for i := 1; i <= len(taskNames); i++ {
		suite.Equal(len(taskNames)-i, suite.is.Queue.Len())
		time.Sleep(time.Duration(offset) * time.Millisecond)
	}
}

func TestIdleSchedulerTestSuite(t *testing.T) {
	suite.Run(t, new(IdleSchedulerTestSuite))
}
