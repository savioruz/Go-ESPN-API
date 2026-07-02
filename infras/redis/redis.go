// Package redis provides Redis client utilities.
//
//nolint:revive
package redis

import (
	"context"
	"crypto/tls"
	"fmt"
	goRedis "github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"
	"go-espn-api/config"
)

// New creates a new Redis client with the provided configuration.
func New(config *config.Config) *goRedis.Client {
	opts := &goRedis.Options{
		Addr:     fmt.Sprintf("%s:%s", config.Cache.Redis.Primary.Host, config.Cache.Redis.Primary.Port),
		Password: config.Cache.Redis.Primary.Password,
		DB:       config.Cache.Redis.Primary.DB,
	}

	if config.Cache.Redis.Primary.TLS {
		opts.TLSConfig = &tls.Config{
			MinVersion: tls.VersionTLS12,
		}
	}

	ctx := context.Background()

	client := goRedis.NewClient(opts)

	_, err := client.Ping(ctx).Result()
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to connect to Redis")
		panic(err)
	}

	log.Info().
		Int("db", config.Cache.Redis.Primary.DB).
		Str("host", config.Cache.Redis.Primary.Host).
		Str("port", config.Cache.Redis.Primary.Port).
		Msg("Connected to Redis")

	return client
}
