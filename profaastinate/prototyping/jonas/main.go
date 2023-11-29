package main

import (
	"container/heap"
	"strconv"
	"time"

	"github.com/Persists/profaastinate-queue/queue/tree"
	"github.com/Persists/profaastinate-queue/task"
)

func main() {

	start := time.Now()

	taskTree := tree.InitTree()

	amount := 1_000_000

	// insert tasks
	for i := 0; i < amount; i++ {
		name := strconv.Itoa(i)

		newTask := task.NewTask(name, start.Add(time.Duration(i)*time.Second))
		taskTree.Insert(newTask)
	}

	popped := taskTree.PopLowestDeadline(amount)

	println("amount popped: ", len(popped))

	println("time elapsed: ", time.Since(start).String())

	startHeap := time.Now()

	newHeap := heap.Init()
	// insert tasks
	for i := 0; i < amount; i++ {
		name := strconv.Itoa(i)
		newHeap.Push()
		heap.Push(taskTree, newTask)
	}

	println("time elapsed: ", time.Since(startHeap).String())

}
