package utils

import (
	"context"
	"github.com/redis/go-redis/v9"
	"log"
)

// PrintTopElements retrieves and prints the top elements from the Redis sorted set.
func PrintTopElements(ctx context.Context, rdb *redis.Client, key RedisKeyEnum, numberOfElements int64) {
	result, err := rdb.ZRevRangeWithScores(ctx, string(key), 0, numberOfElements).Result()
	if err != nil {
		log.Fatalf("Error retrieving %d elements: %v", numberOfElements, err)
	}
	for _, zItem := range result {
		log.Printf("%v\n", zItem)
	}
}
