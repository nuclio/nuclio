package structs

import (
	"net/http"
	"time"
)

type NexusItem struct {
	Request  *http.Request // The value of the NexusEntry; arbitrary.
	Index    int           // The index of the NexusEntry in the heap.
	Deadline time.Time     // The priority of the NexusEntry in the queue.
	Name     string        // The name of the NexusEntry
}

func NewNexusItem(request *http.Request, deadline time.Time, name string) *NexusItem {
	return &NexusItem{
		Request:  request,
		Deadline: deadline,
		Name:     name,
	}
}
