package log

import (
	"log/slog"
	"os"

	"github.com/bry-guy/srvivor/internal/config"
	"github.com/lmittmann/tint"
)

func NewLogger(cfg *config.Config) *slog.Logger {
	var log *slog.Logger
	w := os.Stderr
	log = slog.New(tint.NewHandler(w, &tint.Options{
		Level: cfg.LogLevel,
	}))

	return log
}
