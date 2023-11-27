package internal_heap

import "time"

type Item struct {
	Value         interface{}
	Deadline      time.Time
	OtherPriority int
}

// we need to define a custom type instead of using the raw integer slice
// since we need to define methods on the type to implement the internal-heap interface
type ItemHeap []Item

// Len is the number of elements in the collection.
func (h ItemHeap) Len() int {
	return len(h)
}

// Less reports whether the element with index i
// must sort before the element with index j.
// This will determine whether the internal-heap is a min internal-heap or a max internal-heap
func (h ItemHeap) Less(i int, j int) bool {
	return h[i].OtherPriority > h[j].OtherPriority
}

// Swap swaps the elements with indexes i and j.
func (h ItemHeap) Swap(i int, j int) {
	h[i], h[j] = h[j], h[i]
}

// Push and Pop are used to append and remove the last element of the slice
func (h *ItemHeap) Push(x any) {
	*h = append(*h, x.(Item))
}

func (h *ItemHeap) Pop() any {
	old := *h
	n := len(old)
	x := old[n-1]
	*h = old[0 : n-1]
	return x
}
