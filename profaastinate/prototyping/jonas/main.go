package main

import (
	"time"

	"github.com/Persists/profaastinate-queue/queue"
)

func main() {

	start := time.Now()

	deadlineQueue := queue.InitQueue()

	for i := 0; i < 100000; i++ {
		deadlineQueue.Add(*queue.NewItem("test", time.Now().Add(10*time.Second)))
	}

	deadlineQueue.Pop(time.Now().Add(10 * time.Second))

	println(time.Since(start).String())

}
