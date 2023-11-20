package redis

import (
	"context"
	"fmt"
	"github.com/redis/go-redis/v9"
	"time"
)

type Client struct {
	redis.Client
}

func NewConnectedClient(address string, password string, db int) *Client {
	return &Client{*redis.NewClient(&redis.Options{
		Addr:     address,
		Password: password,
		DB:       db,
	})}
}

func (c *Client) InsertTask(ctx context.Context, task string, deadline int64) error {

	member := &redis.Z{
		Score:  float64(deadline),
		Member: task,
	}

	err := c.ZAdd(ctx, "tasks", *member).Err()

	if err != nil {
		return err
	}

	return nil
}

func (c *Client) PopTaskByTimeOffset(ctx context.Context, offset int) (string, error) {
	timeWithOffset := time.Now().Unix() + int64(offset)
	r := &redis.ZRangeBy{
		Min: fmt.Sprintf("%d", 0),
		Max: fmt.Sprintf("%d", timeWithOffset),
	}

	result, err := c.ZRangeByScore(ctx, "tasks", r).Result()

	if err != nil {
		return "", err
	}

	if len(result) == 0 {
		return "", nil
	}

	err = c.ZRem(ctx, "tasks", result[0]).Err()

	if err != nil {
		return "", err
	}

	return result[0], nil
}

func (c *Client) Close() error {
	return c.Close()
}
