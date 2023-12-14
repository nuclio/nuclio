package scheduler

import (
	"github.com/konsumgandalf/mpga-protoype-david/pkg/nexus/common"
	"github.com/konsumgandalf/mpga-protoype-david/pkg/nexus/deadline/queue"
	"sync"
	"time"
)

type DeadlineScheduler struct {
	mu                       *sync.RWMutex
	queue                    *queue.DeadlineQueue
	deadlineRemovalThreshold time.Duration // Variable determining when a task is removed
	nexus                    *common.Nexus
}

func NewDeadlineScheduler(deadlineRemovalThreshold time.Duration, nexus *common.Nexus) *DeadlineScheduler {
	return &DeadlineScheduler{
		queue:                    queue.NewDeadlineQueue(),
		deadlineRemovalThreshold: deadlineRemovalThreshold,
		nexus:                    nexus,
	}
}

func (ds *DeadlineScheduler) Push(item queue.DeadlineItem) {
	ds.mu.Lock()
	defer ds.mu.Unlock()

	ds.queue.Push(&item)
}

func (ds *DeadlineScheduler) Pop() *queue.DeadlineItem {
	ds.mu.Lock()
	defer ds.mu.Unlock()

	if ds.queue.Len() == 0 {
		return nil
	}

	timeUntilDeadline := ds.queue.Peek().Deadline.Sub(time.Now())
	if timeUntilDeadline.Seconds() < ds.deadlineRemovalThreshold.Seconds() {
		ds.nexus.CallbackRemove(ds, ds.queue.Peek().ID)
		return ds.queue.Pop()
	}

	return nil
}

func (ds *DeadlineScheduler) Remove(itemID string) {
	ds.queue.Remove(itemID)
}
