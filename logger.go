package leaderelection

import (
	"log/slog"
)

type Logger interface {
	Info(msg string, args ...any)
	Warn(msg string, args ...any)
	Error(msg string, args ...any)
	Debug(msg string, args ...any)
}

type defaultLogger struct{}

func (l defaultLogger) Info(msg string, args ...any) {
	slog.Info(msg, args...)
}

func (l defaultLogger) Warn(msg string, args ...any) {
	slog.Warn(msg, args...)
}

func (l defaultLogger) Error(msg string, args ...any) {
	slog.Error(msg, args...)
}

func (l defaultLogger) Debug(msg string, args ...any) {
	slog.Debug(msg, args...)
}

var defaultLog Logger = defaultLogger{}
