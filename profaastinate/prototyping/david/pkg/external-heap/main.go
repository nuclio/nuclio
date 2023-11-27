package main

import (
	"github.com/konsumgandalf/mpga-protoype-david/pkg/internal-heap"
	"github.com/mailgun/holster/v4/collections"
	"math/rand"
	"time"
)

func main() {
	iterations := 100_000
	ownImplementation(iterations)
	usePackage(iterations)
}

func ownImplementation(iterations int) {
	startTime := time.Now()
	h := make(internal_heap.ItemHeap, 0)

	// Add 100,000 elements to the internal-heap
	for i := iterations; i > 0; i-- {
		h.Push(internal_heap.Item{
			Value:         i,
			Deadline:      time.Now().Add(10 * time.Second),
			OtherPriority: i,
		})
	}

	for i := 0; i < iterations; i++ {
		h.Pop()
	}

	// Output the total time taken for all operations
	println(time.Since(startTime).String())
	println(h.Len())
}

func usePackage(iterations int) {
	startTime := time.Now()
	queue := collections.NewPriorityQueue()

	for i := 0; i < iterations; i++ {
		queue.Push(&collections.PQItem{
			Value:    "thing3",
			Priority: int(time.Now().Add(time.Duration(rand.Intn(10000))).Unix()),
		})
	}

	for i := 0; i < iterations; i++ {
		queue.Pop()
	}
	println(time.Since(startTime).String())
	// Output: Item: thing1
}
