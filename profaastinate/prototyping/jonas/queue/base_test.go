package queue

// test & benchmark

import (
	"testing"
	"time"
)

func BenchmarkQueue(b *testing.B) {
	deadlineQueue := InitQueue()
	for i := 0; i < b.N; i++ {
		deadlineQueue.Add(*NewItem("test", time.Now().Add(10*time.Second)))
	}

}
