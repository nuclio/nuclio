package main

import (
	"context"
	"github.com/Persists/profaastinate-queue/queue/redis"
	"time"
)

func main() {
	ctx := context.Background()

	client := redis.NewConnectedClient("localhost:6379", "random-password", 10)

	err := client.InsertTask(ctx, "testTest", time.Now().Unix()+5)

	if err != nil {
		panic(err)
	}

	task, err := client.PopTaskByTimeOffset(ctx, 100)

	if err != nil {
		panic(err)
	}

	println(task)

}
