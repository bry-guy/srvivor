package discord

import (
	"errors"
	"net/http"
	"time"

	"github.com/bry-guy/srvivor/apps/castaway-discord-bot/internal/castaway"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	botCommandsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "castaway_bot_commands_total",
			Help: "Total Castaway Discord bot commands handled.",
		},
		[]string{"group", "command", "result"},
	)
	botCommandDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "castaway_bot_command_duration_seconds",
			Help:    "Castaway Discord bot command duration in seconds.",
			Buckets: []float64{0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10, 20, 30},
		},
		[]string{"group", "command"},
	)
	botDiscordGatewayConnected = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "castaway_bot_discord_gateway_connected",
			Help: "Whether the Castaway Discord bot gateway session is connected (1 = connected, 0 = disconnected).",
		},
	)
)

func observeCommand(command commandSpec, start time.Time, err error) string {
	result := classifyCommandResult(err)
	botCommandsTotal.WithLabelValues(command.group, command.name, result).Inc()
	botCommandDuration.WithLabelValues(command.group, command.name).Observe(time.Since(start).Seconds())
	return result
}

func classifyCommandResult(err error) string {
	if err == nil {
		return "ok"
	}
	var apiErr *castaway.APIError
	if errors.As(err, &apiErr) {
		if apiErr.StatusCode >= http.StatusInternalServerError || apiErr.StatusCode == http.StatusTooManyRequests {
			return "dependency_error"
		}
		return "user_error"
	}
	return "internal_error"
}
