package scheduler

import (
	"nexus/bulk/models"
	common "nexus/common/models"
	"nexus/common/models/configs"
	"nexus/common/models/structs"
	"nexus/nexus"
	"testing"
	"time"
)

func Init(minAmountOfBulkItems int, sleepDuration time.Duration) (*BulkScheduler, *nexus.Nexus) {
	bulkConfig := models.BulkSchedulerConfig{
		MinAmountOfBulkItems: minAmountOfBulkItems,
	}

	baseSchedulerConfig := configs.CreateBaseNexusSchedulerConfig(true, sleepDuration)

	mockNexus := nexus.Init()

	baseScheduler := &common.BaseNexusScheduler{
		Queue:                    mockNexus.Queue,
		BaseNexusSchedulerConfig: baseSchedulerConfig,
	}

	return CreateNewScheduler(baseScheduler, bulkConfig), mockNexus
}

const (
	task_1 = "task1"
	task_2 = "task2"
	task_3 = "task3"
)

func pushTasksToQueue(nexus *nexus.Nexus) {
	//Normally tasks with the same name would have different Values
	task1_1 := &structs.NexusItem{
		Name: task_1,
	}
	nexus.Push(task1_1)
	task1_2 := &structs.NexusItem{
		Name: task_1,
	}
	nexus.Push(task1_2)

	task2_1 := &structs.NexusItem{
		Name: task_2,
	}
	nexus.Push(task2_1)
	task2_2 := &structs.NexusItem{
		Name: task_2,
	}
	nexus.Push(task2_2)

	task3_1 := &structs.NexusItem{
		Name: task_3,
	}
	nexus.Push(task3_1)
}

func TestBulkScheduler(t *testing.T) {
	minAmountOfBulkItems, sleepDuration := 3, 1*time.Millisecond

	bs, mockNexus := Init(minAmountOfBulkItems, sleepDuration)

	pushTasksToQueue(mockNexus)

	// Start scheduling to remove tasks that have passed their deadline
	go bs.Start()

	// Wait for a sufficient time to allow the scheduler to process the tasks
	time.Sleep(1 * time.Millisecond)

	if bs.Queue.Len() != 5 {
		t.Errorf("No item should have been removed since no item meets the target of minAmountOfBulkItems of %d", minAmountOfBulkItems)
		t.Fatalf("Expected the queue to remove 0 items, but it has %d items", bs.Queue.Len())
	}

	mockNexus.Push(&structs.NexusItem{
		Name: task_2,
	})

	if bs.Queue.Len() != 6 {
		t.Errorf("Expected the queue to contain 6 items, but it has %d items", bs.Queue.Len())
	}

	// Wait for a sufficient time to allow the scheduler to process the tasks
	time.Sleep(1 * time.Millisecond)

	if bs.Queue.Len() != 3 {
		t.Errorf("Expected the queue to remove all tasks 2, but it has %d items", bs.Queue.Len())
	}
}
