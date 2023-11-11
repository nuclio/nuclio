package main

import (
	"context"
	"github.com/konsumgandalf/mpga-protoype-david/configs"
	"github.com/konsumgandalf/mpga-protoype-david/pkg/redis/prototyping"
)

func main() {
	ctx := context.Background()

	rdb := configs.SetupRedis(ctx)

	prototyping.TestRedis(ctx, rdb)
}
