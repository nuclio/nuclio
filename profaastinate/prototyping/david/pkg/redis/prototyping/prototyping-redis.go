package prototyping

import (
	"context"
	"github.com/konsumgandalf/mpga-protoype-david/pkg/redis/utils"
	"github.com/redis/go-redis/v9"
	"sync"
)

// TestRedis tests the Redis client.
func TestRedis(ctx context.Context, rdb *redis.Client) {
	iterations := 10000
	numberOfConcurrentRoutines := int(float32(iterations) * 0.05)

	utils.SeedRedisEntries(ctx, rdb, iterations, false)
	var processWg sync.WaitGroup
	utils.ConcurrentlyProcessElements(ctx, rdb, numberOfConcurrentRoutines, &processWg)
	processWg.Wait()

	utils.PrintTopElements(ctx, rdb, utils.TASK, 10)
}
