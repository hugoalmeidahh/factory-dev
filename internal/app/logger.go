package app

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/seuusuario/factorydev/internal/config"
)

func NewLogger(paths *config.Paths, debug bool) (*slog.Logger, error) {
	level := slog.LevelInfo
	if debug {
		level = slog.LevelDebug
	}
	opts := &slog.HandlerOptions{Level: level}

	if debug {
		logger := slog.New(slog.NewTextHandler(os.Stdout, opts))
		slog.SetDefault(logger)
		return logger, nil
	}

	logPath := filepath.Join(paths.Logs, "app.log")
	if err := rotateIfNeeded(logPath, 10*1024*1024); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, fmt.Errorf("abrir log: %w", err)
	}
	logger := slog.New(slog.NewJSONHandler(f, opts))
	slog.SetDefault(logger)
	return logger, nil
}

func rotateIfNeeded(path string, maxSize int64) error {
	st, err := os.Stat(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if st.Size() <= maxSize {
		return nil
	}
	rotated := path + ".1"
	_ = os.Remove(rotated)
	return os.Rename(path, rotated)
}
