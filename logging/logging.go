package logging

import (
	"fmt"
	"log/slog"
	"os"
)

var (
	logger *slog.Logger

	programLevel = new(slog.LevelVar) // Info by default
)

func init() {
	logger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: programLevel}))
}

func SetLevel(level slog.Level) {
	programLevel.Set(level)
}

func Info(a ...any) {
	logger.Info(fmt.Sprint(a...))
}

func Infof(format string, v ...interface{}) {
	logger.Info(fmt.Sprintf(format, v...))
}

func Error(a ...any) {
	logger.Error(fmt.Sprint(a...))
}

func Errorf(format string, v ...interface{}) {
	logger.Error(fmt.Sprintf(format, v...))
}

func Debug(a ...any) {
	logger.Debug(fmt.Sprint(a...))
}

func Debugf(format string, v ...interface{}) {
	logger.Debug(fmt.Sprintf(format, v...))
}
