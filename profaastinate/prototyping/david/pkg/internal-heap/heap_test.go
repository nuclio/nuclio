package internal_heap

import (
	"testing"
	"time"
)

// BenchmarkHeapOperations benchmarks the time taken to add and remove 100,000 elements from a internal-heap.
func BenchmarkHeapOperations(b *testing.B) {

	// Measure the start time
	startTime := time.Now()

	// Create a internal-heap
	h := make(ItemHeap, 0)

	// Add 100,000 elements to the internal-heap
	for i := 0; i < 100000; i++ {
		h.Push(i)
	}

	// Run the benchmark for removing elements from the internal-heap
	for i := 0; i < b.N; i++ {
		for h.Len() > 0 {
			h.Pop()
		}

		// Reset the internal-heap for the next iteration
		h = make(ItemHeap, 0)

		// Add 100,000 elements to the internal-heap again
		for i := 0; i < 100000; i++ {
			h.Push(i)
		}
	}

	// Measure the end time
	elapsedTime := time.Since(startTime)

	// Output the total time taken for all operations
	b.Logf("Total time for all operations: %s", elapsedTime)

}
