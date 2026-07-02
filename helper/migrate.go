// Package helper provides functions to manage database migrations using the golang-migrate library.
package helper

import (
	"errors"
	"fmt"
	"go-espn-api/config"
	"net"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres" // nolint:revive
	_ "github.com/golang-migrate/migrate/v4/source/file"       // nolint:revive
	"github.com/rs/zerolog/log"
)

func getDBName(config *config.Config, baseName string) string {
	if config.DB.Postgres.Prefix != "" {
		return config.DB.Postgres.Prefix + baseName
	}

	return baseName
}

func getConnection(config *config.Config) (*migrate.Migrate, error) {
	connectionString := fmt.Sprintf("postgres://%s:%s@%s/%s?sslmode=%s&x-migrations-table=%s",
		config.DB.Postgres.Write.Username,
		config.DB.Postgres.Write.Password,
		net.JoinHostPort(config.DB.Postgres.Write.Host, config.DB.Postgres.Write.Port),
		getDBName(config, config.DB.Postgres.Write.Name),
		config.DB.Postgres.Write.SSLMode,
		config.DB.Postgres.MigrationTable,
	)

	mig, err := migrate.New(
		"file://migrations/postgres",
		connectionString,
	)
	if err != nil {
		return nil, fmt.Errorf("error creating migrate instance: %w", err)
	}

	return mig, nil
}

// Runner executes the specified migration action (up, down, step-up, drop) using the provided configuration.
func Runner(config *config.Config, action string) error {
	mig, err := getConnection(config)
	if err != nil {
		return fmt.Errorf("error creating migrate instance: %w", err)
	}

	defer func(mig *migrate.Migrate) {
		err, _ := mig.Close()
		if err != nil {
			log.Error().Err(err).Msg("error closing migrate instance")
		}
	}(mig)

	switch action {
	case "up":
		if err := mig.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
			return fmt.Errorf("error running migrations: %w", err)
		}

		log.Info().Msg("Database migrations completed successfully")

		return nil
	case "down":
		if err := mig.Steps(-1); err != nil && !errors.Is(err, migrate.ErrNoChange) {
			return fmt.Errorf("error rolling back migrations: %w", err)
		}

		log.Info().Msg("Database migrations rolled back successfully")

		return nil
	case "step-up":
		if err := mig.Steps(1); err != nil && !errors.Is(err, migrate.ErrNoChange) {
			return fmt.Errorf("error running migrations: %w", err)
		}

		log.Info().Msg("Database migrations completed successfully")

		return nil
	case "drop":
		if err := mig.Down(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
			return fmt.Errorf("error rolling back migrations: %w", err)
		}

		log.Info().Msg("Database migrations rolled back successfully")

		return nil
	}

	return nil
}

// Up applies all available migrations to the database using the provided configuration.
func Up(config *config.Config) error {
	return Runner(config, "up")
}

// StepUp applies the next available migration to the database using the provided configuration.
func StepUp(config *config.Config) error {
	return Runner(config, "step-up")
}

// Down rolls back the last applied migration from the database using the provided configuration.
func Down(config *config.Config) error {
	return Runner(config, "down")
}

// Drop rolls back all applied migrations from the database using the provided configuration.
func Drop(config *config.Config) error {
	return Runner(config, "drop")
}
