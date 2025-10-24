package messager

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
)

type Messager interface {
	Send(message string) error
}

type DiscordMessager struct {
	URL string
}

func NewDiscordMessager(url string) *DiscordMessager {
	return &DiscordMessager{URL: url}
}

func (d *DiscordMessager) Send(message string) error {
	payload := map[string]any{
		"message": message,
	}
	jsonBytes, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	resp, err := http.Post(d.URL, "application/json", bytes.NewBuffer(jsonBytes))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		slog.Warn("Messager responded with non-200", "status", resp.StatusCode)
	}
	return nil
}
