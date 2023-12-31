package deadline

import (
<<<<<<< HEAD
	"github.com/nuclio/nuclio/pkg/nexus/common/models/configs"
	common "github.com/nuclio/nuclio/pkg/nexus/common/models/structs"
	"github.com/nuclio/nuclio/pkg/nexus/common/scheduler"
=======
	"github.com/nuclio/nuclio/pkg/nexus/common/models"
	"github.com/nuclio/nuclio/pkg/nexus/common/models/configs"
	common "github.com/nuclio/nuclio/pkg/nexus/common/models/structs"
>>>>>>> ed6969168 (feat(pkg-restful): nexus)
	deadline "github.com/nuclio/nuclio/pkg/nexus/deadline/models"
	"github.com/nuclio/nuclio/pkg/nexus/nexus"
	"github.com/stretchr/testify/suite"
	"net/http"
	"testing"
	"time"
)

type DeadlineSchedulerTestSuite struct {
	suite.Suite
	DeadlineScheduler *DeadlineScheduler
	MockNexus         *nexus.Nexus
}

func (suite *DeadlineSchedulerTestSuite) SetupTest() {
	deadlineRemovalThreshold, sleepDuration := 2*time.Millisecond, 1*time.Millisecond

	deadlineConfig := deadline.DeadlineSchedulerConfig{
		DeadlineRemovalThreshold: deadlineRemovalThreshold,
	}

	baseSchedulerConfig := configs.NewBaseNexusSchedulerConfig(true, sleepDuration)

	suite.MockNexus = nexus.Initialize()

<<<<<<< HEAD
	baseScheduler := &scheduler.BaseNexusScheduler{
=======
	baseScheduler := &models.BaseNexusScheduler{
>>>>>>> ed6969168 (feat(pkg-restful): nexus)
		Queue:                    suite.MockNexus.Queue,
		BaseNexusSchedulerConfig: baseSchedulerConfig,
	}

	suite.DeadlineScheduler = NewScheduler(baseScheduler, deadlineConfig)
}

func (suite *DeadlineSchedulerTestSuite) TestDeadlineScheduler() {
	mockTask := &common.NexusItem{
		Request:  &http.Request{},
		Deadline: time.Now().Add(2 * time.Millisecond),
	}

	// Push a task to the queue
	suite.MockNexus.Push(mockTask)

	// Start scheduling to remove tasks that have passed their deadline
	go suite.DeadlineScheduler.Start()

	// Wait for a sufficient time to allow the scheduler to process the task
	time.Sleep(suite.DeadlineScheduler.DeadlineRemovalThreshold + 1*time.Millisecond)

	// Push another task to the queue which is expected not to be removed in time since the scheduler currently sleeps for 2 seconds
	suite.MockNexus.Push(mockTask)

	time.Sleep(1 * time.Microsecond)

	// Add assertions or checks based on the expected behavior of your scheduler
	// For example, you can check if the task was removed from the queue as expected
	suite.Equal(1, suite.DeadlineScheduler.Queue.Len())

	time.Sleep(suite.DeadlineScheduler.SleepDuration + 1*time.Millisecond)
	suite.Equal(0, suite.DeadlineScheduler.Queue.Len())

	// Stop the scheduler
	suite.DeadlineScheduler.Stop()
	suite.MockNexus.Push(mockTask)

	time.Sleep(suite.DeadlineScheduler.DeadlineRemovalThreshold + 1*time.Millisecond)
	suite.Equal(1, suite.DeadlineScheduler.Queue.Len())
}

func TestDeadlineSchedulerTestSuite(t *testing.T) {
	suite.Run(t, new(DeadlineSchedulerTestSuite))
}
