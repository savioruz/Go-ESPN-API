// Package config provides a centralized way to manage application configuration settings.
package config

import (
	"fmt"
	"sync"

	"github.com/joho/godotenv"
	"github.com/kelseyhightower/envconfig"
	"github.com/rs/zerolog/log"
)

// Config holds all the configuration settings for the application.
type Config struct {
	Server struct {
		Env      string `envconfig:"ENV"`
		LogLevel string `envconfig:"LOG_LEVEL"`
		Port     string `envconfig:"PORT"`
		Host     string `envconfig:"HOST"`
		Shutdown struct {
			CleanupPeriodSeconds int64 `envconfig:"CLEANUP_PERIOD_SECONDS"`
			GracePeriodSeconds   int64 `envconfig:"GRACE_PERIOD_SECONDS"`
		} `envconfig:"SHUTDOWN"`
	} `envconfig:"SERVER"`

	App struct {
		Name string `envconfig:"APP_NAME"`
		CORS struct {
			AllowCredentials bool     `envconfig:"ALLOW_CREDENTIALS"`
			AllowedHeaders   []string `envconfig:"ALLOWED_HEADERS"`
			AllowedMethods   []string `envconfig:"ALLOWED_METHODS"`
			AllowedOrigins   []string `envconfig:"ALLOWED_ORIGINS"`
			Enable           bool     `envconfig:"ENABLE"`
			MaxAgeSeconds    int      `envconfig:"MAX_AGE_SECONDS"`
		} `envconfig:"CORS"`
		RateLimiter struct {
			Enable        bool `envconfig:"ENABLE"`
			MaxRequests   int  `envconfig:"MAX_REQUESTS"`
			WindowSeconds int  `envconfig:"WINDOW_SECONDS"`
		} `envconfig:"RATE_LIMITER"`
		APIKey string `envconfig:"API_KEY"`
	} `envconfig:"APP"`

	Cache struct {
		Redis struct {
			Primary struct {
				Host     string `envconfig:"HOST"`
				Port     string `envconfig:"PORT"`
				Password string `envconfig:"PASSWORD"`
				DB       int    `envconfig:"DB"`
				TLS      bool   `envconfig:"TLS"`
			} `envconfig:"PRIMARY"`
		} `envconfig:"REDIS"`
		TTL int `envconfig:"TTL"`
	} `envconfig:"CACHE"`

	AuthService struct {
		URL string `envconfig:"URL"`
	} `envconfig:"AUTH_SERVICE"`

	DB struct {
		Postgres struct {
			MaxRetry       int    `default:"3"                 envconfig:"MAX_RETRY"`
			RetryWaitTime  int    `default:"5"                 envconfig:"RETRY_WAIT_TIME"`
			MigrationTable string `envconfig:"MIGRATION_TABLE"`
			AutoMigrate    bool   `envconfig:"AUTO_MIGRATE"`
			Prefix         string `envconfig:"PREFIX"`
			Read           struct {
				Host     string `envconfig:"HOST"`
				Port     string `envconfig:"PORT"`
				Username string `envconfig:"USER"`
				Password string `envconfig:"PASSWORD"`
				Name     string `envconfig:"NAME"`
				Timezone string `envconfig:"TIMEZONE"`
				SSLMode  string `envconfig:"SSL_MODE"`
			} `envconfig:"READ"`
			Write struct {
				Host     string `envconfig:"HOST"`
				Port     string `envconfig:"PORT"`
				Username string `envconfig:"USER"`
				Password string `envconfig:"PASSWORD"`
				Name     string `envconfig:"NAME"`
				Timezone string `envconfig:"TIMEZONE"`
				SSLMode  string `envconfig:"SSL_MODE"`
			} `envconfig:"WRITE"`
		} `envconfig:"POSTGRES"`
	} `envconfig:"DB"`

	Kafka struct {
		SASL struct {
			Username string `envconfig:"USERNAME"`
			Password string `envconfig:"PASSWORD"`
		} `envconfig:"SASL"`
		Brokers       []string `envconfig:"BROKERS"`
		ConsumerGroup string   `envconfig:"CONSUMER_GROUP"`
		Topics        struct{} `envconfig:"TOPICS"`
	} `envconfig:"KAFKA"`

	External struct {
		S3 struct {
			APIEndpoint     string `envconfig:"API_ENDPOINT"`
			AccessKeyID     string `envconfig:"ACCESS_KEY_ID"`
			SecretAccessKey string `envconfig:"SECRET_ACCESS_KEY"`
			BucketName      string `envconfig:"BUCKET_NAME"`
			PublicDomain    string `envconfig:"PUBLIC_DOMAIN"`
		} `envconfig:"S3"`
		Otel struct {
			Endpoint string `envconfig:"ENDPOINT"`
		} `envconfig:"OTEL"`
	} `envconfig:"EXTERNAL"`

	Services struct {
		ESPN struct {
			Enabled         bool     `default:"true"     envconfig:"ENABLED"`
			Relays          []string `envconfig:"RELAYS"`
			Timeout         int      `default:"30"       envconfig:"TIMEOUT"`
			MaxRetries      int      `default:"3"        envconfig:"MAXRETRIES"`
			RetryBackoffMin int      `default:"1"        envconfig:"RETRY_BACKOFF_MIN"`
			RetryBackoffMax int      `default:"10"       envconfig:"RETRY_BACKOFF_MAX"`
			// IngestLeagues is the set of "sport:league" pairs the worker's
			// periodic refresh jobs fan out over. Trimmed to the sports the
			// downstream app uses (soccer incl. World Cup, football, basketball)
			// so each refresh enqueues a handful of ingests, not ~45 — which was
			// spiking Postgres into "too many clients already". Ports base.py's
			// _DEFAULT_INGEST_LEAGUES. Parse with ParseIngestLeagues.
			IngestLeagues []string `default:"soccer:fifa.world,soccer:eng.1,soccer:esp.1,soccer:ger.1,soccer:ita.1,soccer:fra.1,soccer:usa.1,soccer:eng.2,soccer:uefa.champions,soccer:uefa.europa,football:nfl,basketball:nba" envconfig:"INGEST_LEAGUES"`
			// IngestConcurrency bounds the number of concurrent ingest units per
			// job (the "too many clients" fix). Keep it <= the Postgres pool
			// max-open connections (see infras/postgres: postgresMaxOpenConnection).
			IngestConcurrency int `default:"4" envconfig:"INGEST_CONCURRENCY"`
		} `envconfig:"ESPN"`
		NHL struct {
			Enabled    bool `default:"false" envconfig:"ENABLED"`
			Timeout    int  `default:"30"    envconfig:"TIMEOUT"`
			MaxRetries int  `default:"3"     envconfig:"MAXRETRIES"`
			// IngestConcurrency bounds the number of concurrent per-team roster
			// ingests in the NHL rosters sync (the "too many clients" fix). Keep
			// it <= the Postgres pool max-open connections.
			IngestConcurrency int `default:"4" envconfig:"INGEST_CONCURRENCY"`
		} `envconfig:"NHL"`
	} `envconfig:"SERVICES"`
}

var (
	conf        Config
	once        sync.Once
	initialized bool
)

// Init loads the configuration from environment variables and .env file (if present) and processes it into the Config struct.
func Init() error {
	var err error

	once.Do(func() {
		err = godotenv.Load(".env")
		if err != nil {
			log.Warn().Err(err).Msg("Could not load .env file, continuing with existing environment variables")
		} else {
			log.Info().Msg("Successfully loaded variables from .env file into environment")
		}

		err = envconfig.Process("", &conf)
		if err != nil {
			log.Fatal().Err(err).Msg("Failed to process environment variables")
		}

		initialized = true

		log.Info().Msg("Service configuration initialized successfully")
	})

	if err != nil {
		return fmt.Errorf("loading .env file: %w", err)
	}

	return nil
}

// Get returns a pointer to the Config struct. If the configuration has not been initialized yet, it will call Init() to load the configuration first.
func Get() *Config {
	if !initialized {
		if err := Init(); err != nil {
			log.Fatal().Err(err).Msg("Failed to initialize configuration")
		}
	}

	return &conf
}
