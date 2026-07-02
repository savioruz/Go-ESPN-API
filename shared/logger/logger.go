// Package logger provides logging utilities using zerolog.
package logger

import (
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"go-espn-api/config"
	"os"
	"time"
)

// InitLogger initializes the zerolog logger with a console writer and sets the time format to RFC3339.
func InitLogger() {
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix

	zerolog.SetGlobalLevel(zerolog.TraceLevel)

	output := zerolog.ConsoleWriter{Out: os.Stdout, TimeFormat: time.RFC3339}

	log.Logger = log.Output(output)

	log.Trace().Msg("Zerolog initialized.")
}

// ErrorWithStack logs the error with its stack trace.
// It uses the errors.WithStack function to wrap the error and include the stack trace in the log message.
func ErrorWithStack(err error) {
	log.Error().Msgf("%+v", errors.WithStack(err))
}

// SetLogLevel sets the log level based on the configuration. If the log level is not set or invalid, it defaults to TraceLevel.
func SetLogLevel(config *config.Config) {
	level, err := zerolog.ParseLevel(config.Server.LogLevel)
	if err != nil {
		level = zerolog.TraceLevel
		log.Trace().Str("loglevel", level.String()).Msg("Environment has no log level set up, using default.")
	} else {
		log.Trace().Str("loglevel", level.String()).Msg("Desired log level detected.")
	}

	zerolog.SetGlobalLevel(level)
}
