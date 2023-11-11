package main

import (
	"context"
	"fmt"
	"github.com/konsumgandalf/mpga-protoype-david/configs"
	"github.com/konsumgandalf/mpga-protoype-david/pkg/redis/prototyping"
)

func main() {
	ctx := context.Background()

	rdb := configs.SetupRedis(ctx)

	err := rdb.Set(ctx, "key", "value2", 0).Err()
	if err != nil {
		panic(err)
	}

	val, err := rdb.Get(ctx, "key").Result()
	if err != nil {
		panic(err)
	}
	fmt.Println("key", val)

	prototyping.TestRedis(ctx, rdb)
}
