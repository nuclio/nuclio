package queues

import "github.com/konsumgandalf/mpga-protoype-david/pkg/nexus/common/structs"

// type NexusQueue[T classes.BaseNexusItem] []*T

type NexusQueue []*structs.BaseNexusItem

func (nxs NexusQueue) Len() int { return len(nxs) }

func (nxs NexusQueue) Swap(i, j int) {
	nxs[i], nxs[j] = nxs[j], nxs[i]
	nxs[i].Index = i
	nxs[j].Index = j
}

func (nxs *NexusQueue) Push(x any) {
	n := len(*nxs)
	NexusEntry := x.(*structs.BaseNexusItem)
	NexusEntry.Index = n
	*nxs = append(*nxs, NexusEntry)
}

func Pop(nxs *NexusQueue) any {
	old := *nxs
	n := len(old)
	NexusEntry := old[n-1]
	old[n-1] = nil        // avoid memory leak
	NexusEntry.Index = -1 // for safety
	*nxs = old[0 : n-1]
	return NexusEntry
}

func (nxs NexusQueue) Less(i, j int) bool {
	return nxs[i].Index < nxs[j].Index
}
