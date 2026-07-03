package logger

import (
	"os"
	"time"

	"github.com/rs/zerolog"
)

func New(level string) zerolog.Logger {
	parsed, err := zerolog.ParseLevel(level)
	if err != nil {
		parsed = zerolog.InfoLevel
	}

	zerolog.SetGlobalLevel(parsed)
	return zerolog.New(os.Stderr).
		With().
		Timestamp().
		Logger().
		Output(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.RFC3339})
}
