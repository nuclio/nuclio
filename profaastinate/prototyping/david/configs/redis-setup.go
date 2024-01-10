package configs

import (
	"context"
	"github.com/joho/godotenv"
	"github.com/redis/go-redis/extra/redisotel/v9"
	"github.com/redis/go-redis/v9"
	"log"
	"os"
	"strconv"
)

// SetupRedis sets up the redis client based on the env variables
func SetupRedis(ctx context.Context) *redis.Client {
	errEnv := godotenv.Load("envs/.redis.env")
	if errEnv != nil {
		log.Fatal("Error loading .env filed")
	}
	databaseName, _ := strconv.Atoi(os.Getenv("REDIS_DATABASE"))
	poolSize, _ := strconv.Atoi(os.Getenv("REDIS_POOL_SIZE"))

	rdb := redis.NewClient(&redis.Options{
		Addr:     "localhost:" + os.Getenv("REDIS_PORT"),
		Password: os.Getenv("REDIS_PASSWORD"),
		PoolSize: poolSize,
		DB:       databaseName,
	})
	rdb.Ping(ctx)

	registerInstrumentation(rdb)

	return rdb
}

// RegisterInstrumentation registers the OpenTelemetry instrumentation for the Redis client.
func registerInstrumentation(rdb *redis.Client) {
	if err := redisotel.InstrumentTracing(rdb); err != nil {
		panic(err)
	}
	if err := redisotel.InstrumentMetrics(rdb); err != nil {
		panic(err)
	}
}
