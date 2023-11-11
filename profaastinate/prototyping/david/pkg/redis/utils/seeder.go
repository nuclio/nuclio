package utils

import (
	"context"
	"fmt"
	"github.com/redis/go-redis/v9"
	"log"
	"math/rand"
)

// SeedRedisEntries seeds the redis database with simple string entries
func SeedRedisEntries(ctx context.Context, rdb *redis.Client, iterations int, printElements bool) {
	// delete all entries
	rdb.Del(ctx, string(TASK))

	pipe := rdb.TxPipeline()
	tasks := make(map[string]float64)

	for i := 0; i < iterations; i++ {
		task := fmt.Sprintf("task%d", i+1)
		score := float64(rand.Intn(iterations) + 1) // Random score between 1 and 10
		tasks[task] = score
		pipe.ZAdd(ctx, string(TASK), redis.Z{Score: score, Member: task + " - " + fmt.Sprintf("%f", score)})
	}

	// Execute the pipeline
	_, err := pipe.Exec(ctx)
	if err != nil {
		log.Fatal("Pipeline execution error:", err)
	}

	if printElements {
		// Retrieve all elements from the sorted set using ZRange
		result, err := rdb.ZRange(ctx, string(TASK), 0, -1).Result()
		if err != nil {
			log.Fatal("Error retrieving elements:", err)
		}

		// Print the retrieved elements
		for _, member := range result {
			fmt.Println("Task:", member)
		}
	}
}
