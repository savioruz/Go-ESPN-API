package logger_test

import (
	"bytes"
	"errors"
	"go-espn-api/config"
	"go-espn-api/shared/logger"
	"testing"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func TestInitLogger(t *testing.T) {
	// Capture the original logger to restore later
	originalLogger := log.Logger

	// Initialize logger
	logger.InitLogger()

	// Verify that the time field format is set correctly
	if zerolog.TimeFieldFormat != zerolog.TimeFormatUnix {
		t.Errorf("expected TimeFieldFormat to be %s, got %s", zerolog.TimeFormatUnix, zerolog.TimeFieldFormat)
	}

	// Verify that the global level is set to TraceLevel
	if zerolog.GlobalLevel() != zerolog.TraceLevel {
		t.Errorf("expected global level to be %s, got %s", zerolog.TraceLevel, zerolog.GlobalLevel())
	}

	// Restore original logger
	log.Logger = originalLogger
}

func TestErrorWithStack(t *testing.T) {
	// Capture the original logger and set up a buffer to capture output
	originalLogger := log.Logger
	var buf bytes.Buffer
	log.Logger = log.Output(&buf)

	// Test the function
	testErr := errors.New("test error")
	logger.ErrorWithStack(testErr)

	// Check that something was written to the buffer
	output := buf.String()
	if output == "" {
		t.Error("expected error log output, got empty string")
	}

	// Check that the output contains the error message
	if !bytes.Contains(buf.Bytes(), []byte("test error")) {
		t.Error("expected log output to contain 'test error'")
	}

	// Restore original logger
	log.Logger = originalLogger
}

func TestSetLogLevel(t *testing.T) {
	tests := []struct {
		name          string
		logLevel      string
		expectedLevel zerolog.Level
	}{
		{
			name:          "trace level",
			logLevel:      "trace",
			expectedLevel: zerolog.TraceLevel,
		},
		{
			name:          "debug level",
			logLevel:      "debug",
			expectedLevel: zerolog.DebugLevel,
		},
		{
			name:          "info level",
			logLevel:      "info",
			expectedLevel: zerolog.InfoLevel,
		},
		{
			name:          "warn level",
			logLevel:      "warn",
			expectedLevel: zerolog.WarnLevel,
		},
		{
			name:          "error level",
			logLevel:      "error",
			expectedLevel: zerolog.ErrorLevel,
		},
		{
			name:          "fatal level",
			logLevel:      "fatal",
			expectedLevel: zerolog.FatalLevel,
		},
		{
			name:          "panic level",
			logLevel:      "panic",
			expectedLevel: zerolog.PanicLevel,
		},
		{
			name:          "disabled level",
			logLevel:      "disabled",
			expectedLevel: zerolog.Disabled,
		},
		{
			name:          "invalid level defaults to trace",
			logLevel:      "invalid_level",
			expectedLevel: zerolog.TraceLevel,
		},
		{
			name:          "empty level uses NoLevel",
			logLevel:      "",
			expectedLevel: zerolog.NoLevel, // ParseLevel("") returns NoLevel with no error
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Capture the original logger and set up a buffer to capture output
			originalLogger := log.Logger
			var buf bytes.Buffer
			log.Logger = log.Output(&buf)

			// Create test config
			cfg := &config.Config{}
			cfg.Server.LogLevel = tt.logLevel

			// Test the function
			logger.SetLogLevel(cfg)

			// Check that the global level is set correctly
			if zerolog.GlobalLevel() != tt.expectedLevel {
				t.Errorf("expected global level to be %s, got %s", tt.expectedLevel, zerolog.GlobalLevel())
			}

			// Restore original logger
			log.Logger = originalLogger
		})
	}
}

func TestLoggerIntegration(t *testing.T) {
	// Save original state
	originalLogger := log.Logger
	originalLevel := zerolog.GlobalLevel()
	originalTimeFormat := zerolog.TimeFieldFormat

	// Test full initialization flow
	logger.InitLogger()

	cfg := &config.Config{}
	cfg.Server.LogLevel = "info"
	logger.SetLogLevel(cfg)

	// Verify final state
	if zerolog.GlobalLevel() != zerolog.InfoLevel {
		t.Errorf("expected final global level to be %s, got %s", zerolog.InfoLevel, zerolog.GlobalLevel())
	}

	// Test error logging with stack
	var buf bytes.Buffer
	log.Logger = log.Output(&buf)

	testErr := errors.New("integration test error")
	logger.ErrorWithStack(testErr)

	if !bytes.Contains(buf.Bytes(), []byte("integration test error")) {
		t.Error("expected log output to contain 'integration test error'")
	}

	// Restore original state
	log.Logger = originalLogger
	zerolog.SetGlobalLevel(originalLevel)
	zerolog.TimeFieldFormat = originalTimeFormat
}

// Test that logger initialization doesn't panic and sets reasonable defaults
func TestLoggerInitializationStability(t *testing.T) {
	// Save original state
	originalLogger := log.Logger
	originalLevel := zerolog.GlobalLevel()
	originalTimeFormat := zerolog.TimeFieldFormat

	defer func() {
		if r := recover(); r != nil {
			t.Errorf("logger initialization panicked: %v", r)
		}
		// Restore original state
		log.Logger = originalLogger
		zerolog.SetGlobalLevel(originalLevel)
		zerolog.TimeFieldFormat = originalTimeFormat
	}()

	// This should not panic
	logger.InitLogger()

	// Test with various environment scenarios
	testConfigs := []*config.Config{
		func() *config.Config { c := &config.Config{}; c.Server.LogLevel = "info"; return c }(),
		func() *config.Config { c := &config.Config{}; c.Server.LogLevel = "debug"; return c }(),
		func() *config.Config { c := &config.Config{}; c.Server.LogLevel = "error"; return c }(),
		func() *config.Config { c := &config.Config{}; c.Server.LogLevel = "invalid"; return c }(),
		func() *config.Config { c := &config.Config{}; c.Server.LogLevel = ""; return c }(),
	}

	for _, cfg := range testConfigs {
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("SetLogLevel panicked with config %+v: %v", cfg, r)
				}
			}()
			logger.SetLogLevel(cfg)
		}()
	}
}
