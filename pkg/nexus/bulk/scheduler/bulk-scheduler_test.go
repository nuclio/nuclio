package scheduler_test

import (
	"bytes"
	"io"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/nuclio/nuclio/pkg/nexus/bulk/models"
	bulk "github.com/nuclio/nuclio/pkg/nexus/bulk/scheduler"
	"github.com/nuclio/nuclio/pkg/nexus/common/models/config"
	"github.com/nuclio/nuclio/pkg/nexus/common/models/structs"
	common "github.com/nuclio/nuclio/pkg/nexus/common/queue"
	scheduler "github.com/nuclio/nuclio/pkg/nexus/common/scheduler"
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

type MockRoundTripper struct{}

func (m *MockRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	return &http.Response{
		StatusCode: 200,
		Body:       io.NopCloser(bytes.NewBufferString("Mocked response")),
		Header:     make(http.Header),
	}, nil
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

	defaultQueue := common.Initialize()
	baseSchedulerConfig := config.NewBaseNexusSchedulerConfig(true, sleepDuration)
	nexusConfig := config.NewDefaultNexusConfig()

	Client := &http.Client{
		Transport: &MockRoundTripper{},
	}

	baseScheduler := scheduler.
		NewBaseNexusScheduler(defaultQueue, &baseSchedulerConfig, &nexusConfig, Client)

	suite.bs = bulk.NewScheduler(baseScheduler, bulkConfig)
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

	// Set the max parallel requests to 0 to stop the scheduler
	suite.bs.MaxParallelRequests.Store(0)

	for i := 0; i < suite.bs.MinAmountOfBulkItems; i++ {
		suite.bs.Push(&structs.NexusItem{
			Name:    task_2,
			Request: mockRequest,
		})
	}

	suite.Equal(6, suite.bs.Queue.Len())

	// Increase the max parallel requests to 1 to start the scheduler
	suite.bs.MaxParallelRequests.Store(int32(suite.bs.MinAmountOfBulkItems))

	time.Sleep(suite.bs.SleepDuration + 200*time.Millisecond)
	suite.Equal(3, suite.bs.Queue.Len())
}

func TestBulkSchedulerTestSuite(t *testing.T) {
	suite.Run(t, new(BulkSchedulerTestSuite))
}
