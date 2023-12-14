package deadline

import (
	"nexus/common/models"
	"nexus/common/models/configs"
	common "nexus/common/models/structs"
	deadline "nexus/deadline/models"
	"nexus/nexus"
	"testing"
	"time"
)

func Init(deadlineRemovalThreshold time.Duration, sleepDuration time.Duration) (*DeadlineScheduler, *nexus.Nexus) {
	deadlineConfig := deadline.DeadlineSchedulerConfig{
		DeadlineRemovalThreshold: deadlineRemovalThreshold,
	}

	baseSchedulerConfig := configs.CreateBaseNexusSchedulerConfig(true, sleepDuration)

	mockNexus := nexus.Init()

	baseScheduler := &models.BaseNexusScheduler{
		Queue:                    mockNexus.Queue,
		BaseNexusSchedulerConfig: baseSchedulerConfig,
	}

	ds := CreateNewScheduler(baseScheduler, deadlineConfig)

	return ds, mockNexus
}

func TestDeadlineScheduler(t *testing.T) {
	deadlineRemovalThreshold, sleepDuration := 2*time.Millisecond, 1*time.Millisecond

	ds, mockNexus := Init(deadlineRemovalThreshold, sleepDuration)

	mockTask := &common.NexusItem{
		Value:    "test",
		Deadline: time.Now().Add(2 * time.Millisecond),
	}

	// Push a task to the queue
	mockNexus.Push(mockTask)

	// Start scheduling to remove tasks that have passed their deadline
	go ds.Start()

	// Wait for a sufficient time to allow the scheduler to process the task
	time.Sleep(deadlineRemovalThreshold + 1*time.Millisecond)

	// Push another task to the queue which is expected not to be remove in time since the scheduler currently sleeps for 2 seconds
	mockNexus.Push(mockTask)

	time.Sleep(1 * time.Microsecond)

	// Add assertions or checks based on the expected behavior of your scheduler
	// For example, you can check if the task was removed from the queue as expected
	if ds.Queue.Len() != 1 {
		t.Fatalf("Expected the queue to contain 1 item, but it has %d items", ds.Queue.Len())
	}

	time.Sleep(sleepDuration + 1*time.Millisecond)
	if ds.Queue.Len() != 0 {
		t.Fatalf("Expected the queue to now contain 0 items, but it has %d items", ds.Queue.Len())
	}

	// Stop the scheduler
	ds.Stop()
	mockNexus.Push(mockTask)

	time.Sleep(deadlineRemovalThreshold + 1*time.Millisecond)
	if ds.Queue.Len() != 1 {
		t.Fatalf("Expected the queue to contain 1 item, but it has %d items", ds.Queue.Len())
	}
}
