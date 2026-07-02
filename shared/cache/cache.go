// Package cache provides caching utilities using Redis.
package cache

//go:generate go run go.uber.org/mock/mockgen -source=./cache.go -destination=./mocks/cache_mock.go -package=mocks

import (
	"context"
	"encoding/json"
	"fmt"
	"go-espn-api/infras/otel"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"
)

const (
	otelScopeName         = "cache"
	otelCacheKeyAttribute = "cache.key"
	// Nil is abstracted from redis.Nil to avoid direct dependency on the redis package in other parts of the codebase.
	Nil = redis.Nil
)

// RedisCache defines the interface for caching operations using Redis.
type RedisCache interface {
	Save(ctx context.Context, key string, value any, duration int) (err error)
	Get(ctx context.Context, key string, value any) (err error)
	Delete(ctx context.Context, key string) error
	Clear(ctx context.Context, prefix string) error
}

type redisCache struct {
	client *redis.Client
	otel   otel.Otel
}

// NewRedisCache creates a new instance of RedisCache with the provided Redis client and OpenTelemetry instance.
func NewRedisCache(client *redis.Client, ot otel.Otel) RedisCache {
	return &redisCache{
		client: client,
		otel:   ot,
	}
}

// Clear implements RedisCache.
func (cache *redisCache) Clear(ctx context.Context, prefix string) (err error) {
	ctx, scope := cache.otel.NewScope(ctx, otelScopeName, otelScopeName+".Clear")
	defer scope.End()
	defer scope.TraceIfError(err)

	scope.SetAttribute(otelCacheKeyAttribute, prefix)

	scan := cache.client.Scan(ctx, 0, prefix, 0)
	if scan != nil {
		iter := scan.Iterator()

		for iter.Next(ctx) {
			key := iter.Val()
			if err = cache.client.Del(ctx, key).Err(); err != nil {
				log.Error().Err(err).Str("key", key).Str("RedisCache", "Clear").Msg("failed to del cache")

				return fmt.Errorf("failed to delete cache value: %w", err)
			}
		}
	}

	return nil
}

// Delete implements RedisCache.
func (cache *redisCache) Delete(ctx context.Context, key string) (err error) {
	ctx, scope := cache.otel.NewScope(ctx, otelScopeName, otelScopeName+".Delete")
	defer scope.End()
	defer scope.TraceIfError(err)

	scope.SetAttribute(otelCacheKeyAttribute, key)

	if err = cache.client.Del(ctx, key).Err(); err != nil {
		log.Error().Str("key", key).Err(err).Str("RedisCache", "Delete").Msg("failed to del cache")

		return fmt.Errorf("failed to delete cache value: %w", err)
	}

	return nil
}

// Get implements RedisCache.
func (cache *redisCache) Get(ctx context.Context, key string, value any) (err error) {
	ctx, scope := cache.otel.NewScope(ctx, otelScopeName, otelScopeName+".Get")
	defer scope.End()
	defer scope.TraceIfError(err)

	scope.SetAttribute(otelCacheKeyAttribute, key)

	cacheValue, err := cache.client.Get(ctx, key).Result()
	if err == nil {
		switch v := value.(type) {
		case *string:
			*v = cacheValue
		default:
			err = json.Unmarshal([]byte(cacheValue), value)
			if err != nil {
				log.Error().Err(err).Str("RedisCache", "Get").Msg("failed to unmarshal cache")

				return fmt.Errorf("failed to unmarshal cache value: %w", err)
			}

			return nil
		}
	}

	return fmt.Errorf("failed to get cache value: %w", err)
}

// Save implements RedisCache.
func (cache *redisCache) Save(ctx context.Context, key string, value any, duration int) (err error) {
	ctx, scope := cache.otel.NewScope(ctx, otelScopeName, otelScopeName+".Save")
	defer scope.End()
	defer scope.TraceIfError(err)

	scope.SetAttribute(otelCacheKeyAttribute, key)

	var strValue []byte

	switch v := value.(type) {
	case string:
		strValue = []byte(v)
	default:
		strValue, err = json.Marshal(v)
		if err != nil {
			scope.TraceError(err)
			log.Error().Err(err).Str("key", key).Str("RedisCache", "Save").Msg("failed to marshal cache")

			return fmt.Errorf("failed to marshal cache value: %w", err)
		}
	}

	err = cache.client.Set(ctx, key, strValue, time.Second*time.Duration(duration)).Err()
	if err != nil {
		scope.TraceError(err)

		log.Error().Err(err).Str("key", key).Str("RedisCache", "Save").Msg("failed to set cache")

		return fmt.Errorf("failed to set cache value: %w", err)
	}

	log.Info().Str("RedisCache", "Save").Str("key", key).Msg("success to set cache")

	return nil
}
