package scheduler

import (
	"github.com/nuclio/nuclio/pkg/nexus/bulk/models"
<<<<<<< HEAD
<<<<<<< HEAD
	"github.com/nuclio/nuclio/pkg/nexus/common/models/configs"
	"github.com/nuclio/nuclio/pkg/nexus/common/models/structs"
	common "github.com/nuclio/nuclio/pkg/nexus/common/scheduler"
=======
	common "github.com/nuclio/nuclio/pkg/nexus/common/models"
	"github.com/nuclio/nuclio/pkg/nexus/common/models/configs"
	"github.com/nuclio/nuclio/pkg/nexus/common/models/structs"
>>>>>>> ed6969168 (feat(pkg-restful): nexus)
=======
	"github.com/nuclio/nuclio/pkg/nexus/common/models/configs"
	"github.com/nuclio/nuclio/pkg/nexus/common/models/structs"
	common "github.com/nuclio/nuclio/pkg/nexus/common/scheduler"
>>>>>>> bbe05e095 (feat(pkg-nexus): models, scheduler, utils)
	"github.com/nuclio/nuclio/pkg/nexus/nexus"
	"github.com/stretchr/testify/suite"
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
	BulkScheduler *BulkScheduler
	MockNexus     *nexus.Nexus
}

func (suite *BulkSchedulerTestSuite) SetupTest() {
	minAmountOfBulkItems, sleepDuration := 3, 1*time.Millisecond

	bulkConfig := models.BulkSchedulerConfig{
		MinAmountOfBulkItems: minAmountOfBulkItems,
	}

	baseSchedulerConfig := configs.NewBaseNexusSchedulerConfig(true, sleepDuration)

	suite.MockNexus = nexus.Initialize()

	baseScheduler := &common.BaseNexusScheduler{
		Queue:                    suite.MockNexus.Queue,
		BaseNexusSchedulerConfig: baseSchedulerConfig,
	}

	suite.BulkScheduler = NewScheduler(baseScheduler, bulkConfig)
}

func (suite *BulkSchedulerTestSuite) pushTasksToQueue() {
	// Normally tasks with the same name would have different Values
	task1_1 := &structs.NexusItem{
		Name: task_1,
	}
	suite.MockNexus.Push(task1_1)
	task1_2 := &structs.NexusItem{
		Name: task_1,
	}
	suite.MockNexus.Push(task1_2)

	task2_1 := &structs.NexusItem{
		Name: task_2,
	}
	suite.MockNexus.Push(task2_1)
	task2_2 := &structs.NexusItem{
		Name: task_2,
	}
	suite.MockNexus.Push(task2_2)

	task3_1 := &structs.NexusItem{
		Name: task_3,
	}
	suite.MockNexus.Push(task3_1)
}

func (suite *BulkSchedulerTestSuite) TestBulkScheduler() {
	suite.pushTasksToQueue()

	// Start scheduling to remove tasks that have passed their deadline
	go suite.BulkScheduler.Start()

	// Wait for a sufficient time to allow the scheduler to process the tasks
	time.Sleep(1 * time.Millisecond)

	suite.Equal(5, suite.BulkScheduler.Queue.Len())

	suite.MockNexus.Push(&structs.NexusItem{
		Name: task_2,
	})

	suite.Equal(6, suite.BulkScheduler.Queue.Len())

	// Wait for a sufficient time to allow the scheduler to process the tasks
	time.Sleep(1 * time.Millisecond)

	suite.Equal(3, suite.BulkScheduler.Queue.Len())
}

func TestBulkSchedulerTestSuite(t *testing.T) {
	suite.Run(t, new(BulkSchedulerTestSuite))
}
