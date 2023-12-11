package common

import (
	"github.com/konsumgandalf/mpga-protoype-david/pkg/nexus/common/interfaces"
	"github.com/konsumgandalf/mpga-protoype-david/pkg/nexus/common/structs"
	"sync"
)

type Nexus struct {
	nexusScheduler map[string]interfaces.INexusScheduler
	mu             *sync.RWMutex
}

func (nxs *Nexus) CallbackRemove(callingSchedular interface{}, itemID string) {
	callbackRemoveFunction := func(wg *sync.WaitGroup) {

		for schedulerName, scheduler := range nxs.nexusScheduler {
			// Skip the Scheduler that initiated the callback
			if schedulerName == GetStructName(callingSchedular) {
				continue
			}

			wg.Add(1)
			go func(s interfaces.INexusScheduler) {
				defer wg.Done()
				s.Remove(itemID)
			}(scheduler)
		}
	}

	nxs.SynchronizedOperation(callbackRemoveFunction)
}

func (nxs *Nexus) Push(item structs.BaseNexusItem) {
	pushFunction := func(wg *sync.WaitGroup) {

		for _, scheduler := range nxs.nexusScheduler {
			wg.Add(1)
			go func(s interfaces.INexusScheduler) {
				defer wg.Done()
				s.Push(item)
			}(scheduler)
		}
	}

	nxs.SynchronizedOperation(pushFunction)
}

func (nxs *Nexus) SynchronizedOperation(operations func(*sync.WaitGroup)) {
	nxs.mu.Lock()
	defer nxs.mu.Unlock()

	wg := sync.WaitGroup{}

	operations(&wg)

	wg.Wait() // Wait for all operations to complete
}
