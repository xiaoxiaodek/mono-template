package logger

import (
	"io"
	"os"

	"github.com/rs/zerolog"
)

// New creates the application logger. Only local development uses the
// human-readable console writer; every other environment emits JSON.
func New(env string) zerolog.Logger {
	return NewWithWriter(env, os.Stdout)
}

// NewWithWriter is New with an injectable output, primarily for tests and
// embedders that manage their own log destination.
func NewWithWriter(env string, output io.Writer) zerolog.Logger {
	level := zerolog.InfoLevel
	writer := output
	if env == "dev" {
		level = zerolog.DebugLevel
		writer = zerolog.ConsoleWriter{Out: output}
	} else if env == "test" {
		level = zerolog.DebugLevel
	}

	return zerolog.New(writer).Level(level).With().Timestamp().Logger()
}
