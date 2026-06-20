package redis

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/go-redis/redis/v8"

	"cx6/internal/config"
)

var Client *redis.Client

func Init(cfg *config.RedisConfig) error {
	Client = redis.NewClient(&redis.Options{
		Addr:         cfg.Addr(),
		Password:     cfg.Password,
		DB:           cfg.DB,
		PoolSize:     cfg.PoolSize,
		MinIdleConns: cfg.MinIdleConns,
		MaxRetries:   cfg.MaxRetries,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := Client.Ping(ctx).Err(); err != nil {
		return fmt.Errorf("redis ping failed: %w", err)
	}

	log.Printf("redis connected successfully at %s", cfg.Addr())
	return nil
}

func ZIncrBy(ctx context.Context, key string, increment float64, member string) (float64, error) {
	return Client.ZIncrBy(ctx, key, increment, member).Result()
}

func ZRevRangeWithScores(ctx context.Context, key string, start, stop int64) ([]redis.Z, error) {
	return Client.ZRevRangeWithScores(ctx, key, start, stop).Result()
}

func ZRevRank(ctx context.Context, key string, member string) (int64, error) {
	return Client.ZRevRank(ctx, key, member).Result()
}

func ZScore(ctx context.Context, key string, member string) (float64, error) {
	return Client.ZScore(ctx, key, member).Result()
}

func ZCard(ctx context.Context, key string) (int64, error) {
	return Client.ZCard(ctx, key).Result()
}

func IncrBy(ctx context.Context, key string, value int64) (int64, error) {
	return Client.IncrBy(ctx, key, value).Result()
}

func Expire(ctx context.Context, key string, expiration time.Duration) (bool, error) {
	return Client.Expire(ctx, key, expiration).Result()
}

func Get(ctx context.Context, key string) (string, error) {
	return Client.Get(ctx, key).Result()
}

func SetNX(ctx context.Context, key string, value interface{}, expiration time.Duration) (bool, error) {
	return Client.SetNX(ctx, key, value, expiration).Result()
}

func Close() error {
	if Client != nil {
		return Client.Close()
	}
	return nil
}
