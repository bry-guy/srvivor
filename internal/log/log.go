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

	// // Log config out when level is Debug
	// // Config can't log itself, since it's the root configurer
	// if cfg.LogLevel == slog.LevelDebug {
	// 	log.Debug("log.NewLogger: ", "config", fmt.Sprintf("%+v", cfg))
	// }
	return log
}
