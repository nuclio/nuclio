package scheduler

import (
	"github.com/nuclio/nuclio/pkg/nexus/bulk/models"
	"github.com/nuclio/nuclio/pkg/nexus/common/models/configs"
	"github.com/nuclio/nuclio/pkg/nexus/common/models/structs"
	common "github.com/nuclio/nuclio/pkg/nexus/common/queue"
	scheduler "github.com/nuclio/nuclio/pkg/nexus/common/scheduler"
	"github.com/stretchr/testify/suite"
	"net/http"
	"net/url"
	"testing"
	"time"
)

const (
	task_1 = "task1"
	task_2 = "task2"
	task_3 = "task3"
)

type BulkSchedulerTestSuite struct {
	suite.Suite
	bs *BulkScheduler
}

var mockRequest = &http.Request{
	Method: "GET",
	URL: &url.URL{
		Path:   "/api",
		Scheme: "http",
		Host:   "localhost:8070",
	},
	Header: make(http.Header),
}

func (suite *BulkSchedulerTestSuite) SetupTest() {
	minAmountOfBulkItems, sleepDuration := 3, 1*time.Millisecond

	bulkConfig := models.BulkSchedulerConfig{
		MinAmountOfBulkItems:  minAmountOfBulkItems,
		MaxPercentageUsageCPU: 90,
		MaxPercentageUsageRAM: 90,
	}

	baseSchedulerConfig := configs.NewBaseNexusSchedulerConfig(true, sleepDuration)
	defaultQueue := common.Initialize()

	baseScheduler := scheduler.NewBaseNexusScheduler(defaultQueue, baseSchedulerConfig)

	suite.bs = NewScheduler(baseScheduler, bulkConfig)
}

func (suite *BulkSchedulerTestSuite) pushTasksToQueue() {
	// Normally tasks with the same name would have different Values
	task1_1 := &structs.NexusItem{
		Name:    task_1,
		Request: mockRequest,
	}
	suite.bs.Push(task1_1)
	task1_2 := &structs.NexusItem{
		Name:    task_1,
		Request: mockRequest,
	}
	suite.bs.Push(task1_2)

	task2_1 := &structs.NexusItem{
		Name:    task_2,
		Request: mockRequest,
	}
	suite.bs.Push(task2_1)
	task2_2 := &structs.NexusItem{
		Name:    task_2,
		Request: mockRequest,
	}
	suite.bs.Push(task2_2)

	task3_1 := &structs.NexusItem{
		Name:    task_3,
		Request: mockRequest,
	}
	suite.bs.Push(task3_1)
}

func (suite *BulkSchedulerTestSuite) TestBulkScheduler() {
	suite.pushTasksToQueue()

	// Start scheduling to remove tasks that have passed their deadline
	go suite.bs.Start()

	// Wait for a sufficient time to allow the scheduler to process the tasks
	time.Sleep(1 * time.Millisecond)

	suite.Equal(5, suite.bs.Queue.Len())

	suite.bs.Push(&structs.NexusItem{
		Name:    task_2,
		Request: mockRequest,
	})

	suite.Equal(6, suite.bs.Queue.Len())

	// Wait for a sufficient time to allow the scheduler to process the tasks
	time.Sleep(20 * time.Millisecond)

	suite.Equal(3, suite.bs.Queue.Len())
}

func TestBulkSchedulerTestSuite(t *testing.T) {
	suite.Run(t, new(BulkSchedulerTestSuite))
}
