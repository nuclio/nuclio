package deadline_test

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/nuclio/nuclio/pkg/nexus/common/models/config"
	common "github.com/nuclio/nuclio/pkg/nexus/common/queue"
	"github.com/nuclio/nuclio/pkg/nexus/common/scheduler"
	"github.com/nuclio/nuclio/pkg/nexus/deadline/models"
	deadline "github.com/nuclio/nuclio/pkg/nexus/deadline/scheduler"
	utils "github.com/nuclio/nuclio/pkg/nexus/utils"
	"github.com/stretchr/testify/suite"
)

type DeadlineSchedulerTestSuite struct {
	suite.Suite
	ds *deadline.DeadlineScheduler
}

type MockDeployer struct{}

func (bns *MockDeployer) Unpause(functionName string) {
	fmt.Printf("Unpausing function %s\n", functionName)
}

func (suite *DeadlineSchedulerTestSuite) SetupTest() {
	deadlineRemovalThreshold, sleepDuration := 2*time.Millisecond, 1*time.Millisecond

	deadlineConfig := models.DeadlineSchedulerConfig{
		DeadlineRemovalThreshold: deadlineRemovalThreshold,
	}

	defaultQueue := common.Initialize()
	baseSchedulerConfig := config.NewBaseNexusSchedulerConfig(true, sleepDuration)
	nexusConfig := config.NewDefaultNexusConfig()

	Client := &http.Client{
		Transport: &utils.MockRoundTripper{},
	}

	baseScheduler := scheduler.NewBaseNexusScheduler(defaultQueue, &baseSchedulerConfig, &nexusConfig, Client, nil, nil)

	suite.ds = deadline.NewScheduler(baseScheduler, deadlineConfig)
}

func (suite *DeadlineSchedulerTestSuite) TestDeadlineScheduler() {

	// Push a task to the queue
	utils.PushMockedTasksToQueue(&suite.ds.BaseNexusScheduler, []string{"task1"}, 2)

	// Start scheduling to remove tasks that have passed their deadline
	go suite.ds.Start()

	// Wait for a sufficient time to allow the scheduler to process the task
	time.Sleep(suite.ds.DeadlineRemovalThreshold + 1*time.Millisecond)

	// Push another task to the queue which is expected not to be removed in time since the scheduler currently sleeps for 2 seconds
	utils.PushMockedTasksToQueue(&suite.ds.BaseNexusScheduler, []string{"task1"}, 2)

	time.Sleep(1 * time.Microsecond)

	// Add assertions or checks based on the expected behavior of your scheduler
	// For example, you can check if the task was removed from the queue as expected
	suite.Equal(1, suite.ds.Queue.Len())

	time.Sleep(suite.ds.SleepDuration + 300*time.Millisecond)
	suite.Equal(0, suite.ds.Queue.Len())

	// Pause the scheduler
	suite.ds.Stop()
	utils.PushMockedTasksToQueue(&suite.ds.BaseNexusScheduler, []string{"task1"}, 2)

	time.Sleep(suite.ds.DeadlineRemovalThreshold + 200*time.Millisecond)
	suite.Equal(1, suite.ds.Queue.Len())
}

func TestDeadlineSchedulerTestSuite(t *testing.T) {
	suite.Run(t, new(DeadlineSchedulerTestSuite))
}
