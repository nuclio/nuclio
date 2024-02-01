package scheduler_test

import (
	"net/http"
	"testing"
	"time"

	"github.com/nuclio/nuclio/pkg/nexus/bulk/models"
	bulk "github.com/nuclio/nuclio/pkg/nexus/bulk/scheduler"
	"github.com/nuclio/nuclio/pkg/nexus/common/models/config"
	common "github.com/nuclio/nuclio/pkg/nexus/common/queue"
	"github.com/nuclio/nuclio/pkg/nexus/common/scheduler"
	utils "github.com/nuclio/nuclio/pkg/nexus/utils"
	"github.com/stretchr/testify/suite"
)

const (
	task_1 = "task1"
	task_2 = "task2"
	task_3 = "task3"
)

type BulkSchedulerTestSuite struct {
	suite.Suite
	bs *bulk.BulkScheduler
}

func (suite *BulkSchedulerTestSuite) SetupTest() {
	minAmountOfBulkItems, sleepDuration := 3, 1*time.Millisecond

	bulkConfig := models.BulkSchedulerConfig{
		MinAmountOfBulkItems: minAmountOfBulkItems,
	}

	defaultQueue := common.Initialize()
	baseSchedulerConfig := config.NewBaseNexusSchedulerConfig(true, sleepDuration)
	nexusConfig := config.NewDefaultNexusConfig()

	Client := &http.Client{
		Transport: &utils.MockRoundTripper{},
	}

	baseScheduler := scheduler.NewBaseNexusScheduler(defaultQueue, &baseSchedulerConfig, &nexusConfig, Client, nil, nil)

	suite.bs = bulk.NewScheduler(baseScheduler, bulkConfig)
}

func (suite *BulkSchedulerTestSuite) TestBulkScheduler() {

	names := []string{
		task_1,
		task_1,
		task_2,
		task_2,
		task_3,
	}

	utils.PushMockedTasksToQueue(&suite.bs.BaseNexusScheduler, names, 2)

	// Start scheduling to remove tasks that have passed their deadline
	go suite.bs.Start()

	// Wait for a sufficient time to allow the scheduler to process the tasks
	time.Sleep(1 * time.Millisecond)

	suite.Equal(5, suite.bs.Queue.Len())

	utils.PushMockedTasksToQueue(&suite.bs.BaseNexusScheduler, []string{task_2}, 2)

	suite.Equal(6, suite.bs.Queue.Len())

	// Wait for a sufficient time to allow the scheduler to process the tasks
	time.Sleep(20 * time.Millisecond)

	suite.Equal(3, suite.bs.Queue.Len())

	// Set the max parallel requests to 0 to stop the scheduler
	suite.bs.MaxParallelRequests.Store(0)

	for i := 0; i < suite.bs.MinAmountOfBulkItems; i++ {
		utils.PushMockedTasksToQueue(&suite.bs.BaseNexusScheduler, []string{task_2}, 2)
	}

	suite.Equal(6, suite.bs.Queue.Len())

	// Wait again since not enough requests should be available
	suite.bs.MaxParallelRequests.Store(int32(suite.bs.MinAmountOfBulkItems) + suite.bs.CurrentParallelRequests.Load())

	// Continue scheduling by making enough requests available
	suite.bs.MaxParallelRequests.Store(int32(suite.bs.MinAmountOfBulkItems) + suite.bs.CurrentParallelRequests.Load() + 1)

	time.Sleep(suite.bs.SleepDuration + 200*time.Millisecond)
	suite.Equal(3, suite.bs.Queue.Len())
}

func TestBulkSchedulerTestSuite(t *testing.T) {
	suite.Run(t, new(BulkSchedulerTestSuite))
}
