package deadline

import (
	"fmt"
	"github.com/nuclio/nuclio/pkg/nexus/common/models/config"
	structsCommon "github.com/nuclio/nuclio/pkg/nexus/common/models/structs"
	common "github.com/nuclio/nuclio/pkg/nexus/common/queue"
	"github.com/nuclio/nuclio/pkg/nexus/common/scheduler"
	deadline "github.com/nuclio/nuclio/pkg/nexus/deadline/models"
	"github.com/stretchr/testify/suite"
	"net/http"
	"net/url"
	"testing"
	"time"
)

type DeadlineSchedulerTestSuite struct {
	suite.Suite
	ds *DeadlineScheduler
}

type MockDeployer struct{}

func (bns *MockDeployer) Unpause(functionName string) {
	fmt.Printf("Unpausing function %s\n", functionName)
}

func (suite *DeadlineSchedulerTestSuite) SetupTest() {
	deadlineRemovalThreshold, sleepDuration := 2*time.Millisecond, 1*time.Millisecond

	deadlineConfig := deadline.DeadlineSchedulerConfig{
		DeadlineRemovalThreshold: deadlineRemovalThreshold,
	}

	defaultQueue := common.Initialize()
	baseSchedulerConfig := config.NewBaseNexusSchedulerConfig(true, sleepDuration)
	nexusConfig := config.NewDefaultNexusConfig()

	baseScheduler := scheduler.NewBaseNexusScheduler(defaultQueue, &baseSchedulerConfig, &nexusConfig, nil)

	suite.ds = NewScheduler(baseScheduler, deadlineConfig)
}

func (suite *DeadlineSchedulerTestSuite) TestDeadlineScheduler() {
	mockTask := &structsCommon.NexusItem{
		Request: &http.Request{
			Method: "GET",
			URL: &url.URL{
				Path:   "/api",
				Scheme: "http",
				Host:   "localhost:8070",
			},
			Header: make(http.Header),
		},
		Deadline: time.Now().Add(2 * time.Millisecond),
	}

	// Push a task to the queue
	suite.ds.Push(mockTask)

	// Start scheduling to remove tasks that have passed their deadline
	go suite.ds.Start()

	// Wait for a sufficient time to allow the scheduler to process the task
	time.Sleep(suite.ds.DeadlineRemovalThreshold + 1*time.Millisecond)

	// Push another task to the queue which is expected not to be removed in time since the scheduler currently sleeps for 2 seconds
	suite.ds.Push(mockTask)

	time.Sleep(1 * time.Microsecond)

	// Add assertions or checks based on the expected behavior of your scheduler
	// For example, you can check if the task was removed from the queue as expected
	suite.Equal(1, suite.ds.Queue.Len())

	time.Sleep(suite.ds.SleepDuration + 200*time.Millisecond)
	suite.Equal(0, suite.ds.Queue.Len())

	// Pause the scheduler
	suite.ds.Stop()
	suite.ds.Push(mockTask)

	time.Sleep(suite.ds.DeadlineRemovalThreshold + 200*time.Millisecond)
	suite.Equal(1, suite.ds.Queue.Len())
}

func TestDeadlineSchedulerTestSuite(t *testing.T) {
	suite.Run(t, new(DeadlineSchedulerTestSuite))
}
