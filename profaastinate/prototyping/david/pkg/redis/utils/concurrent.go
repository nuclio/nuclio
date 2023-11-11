package utils

import (
	"context"
	"fmt"
	"github.com/redis/go-redis/v9"
	"log"
	"sync"
)

// ConcurrentlyProcessElements concurrently processes elements from the Redis sorted set.
func ConcurrentlyProcessElements(ctx context.Context, rdb *redis.Client, numberOfConcurrentRoutines int, taskWg *sync.WaitGroup) {
	taskWg.Add(numberOfConcurrentRoutines)

	resultCh := make(chan string)

	for i := 0; i < numberOfConcurrentRoutines; i++ {
		go processElement(ctx, rdb, string(TASK), resultCh, taskWg)
	}

	go func() {
		for message := range resultCh {
			fmt.Println(message)
		}

		taskWg.Wait()
		close(resultCh)
	}()
}

// processElement processes a single element from the Redis sorted set.
func processElement(ctx context.Context, rdb *redis.Client, key string, resultCh chan<- string, wg *sync.WaitGroup) {
	defer wg.Done()

	result, err := rdb.ZPopMax(ctx, key).Result()

	if err != nil {
		log.Printf("Error processing element: %v", err)
		return
	}

	resultCh <- fmt.Sprintf("Removed Task: %s, Score: %f", result[0].Member, result[0].Score)
}
