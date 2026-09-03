package goredisv8

import (
	"context"

	"github.com/go-redis/redis/v8"
	"libx.net/workerid"
)

// NewDoer adapts a go-redis v8 UniversalClient to workerid.RedisDoer.
func NewDoer(client redis.UniversalClient) workerid.RedisDoer {
	return workerid.RedisFunc(func(ctx context.Context, args ...any) (any, error) {
		return client.Do(ctx, args...).Result()
	})
}

// NewGenerator creates a RedisGenerator backed by go-redis v8.
func NewGenerator(client redis.UniversalClient, cluster string, options ...workerid.Option) (*workerid.RedisGenerator, error) {
	return workerid.NewRedisGenerator(NewDoer(client), cluster, options...)
}
