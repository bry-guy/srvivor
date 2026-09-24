package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

type config struct {
	APIURL          string `json:"api_url"`
	DiscordUserID   string `json:"discord_user_id"`
	Token           string `json:"token"`
	DiscordBotToken string `json:"discord_bot_token"`
}

func loadConfig() (config, error) {
	var cfg config
	home, err := os.UserHomeDir()
	if err != nil {
		return cfg, fmt.Errorf("cannot locate home directory for Probst config")
	}
	path := filepath.Join(home, ".config", "probst", "config.json")
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return cfg, nil
	}
	if err != nil {
		return cfg, fmt.Errorf("cannot inspect Probst config: %w", err)
	}
	if !info.Mode().IsRegular() || info.Mode().Perm()&0o077 != 0 {
		return cfg, fmt.Errorf("Probst config must be a regular file accessible only to its owner (chmod 600)")
	}
	file, err := os.Open(path)
	if err != nil {
		return cfg, fmt.Errorf("cannot open Probst config: %w", err)
	}
	defer file.Close()
	info, err = file.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0o077 != 0 {
		return cfg, fmt.Errorf("Probst config must be a regular file accessible only to its owner (chmod 600)")
	}
	decoder := json.NewDecoder(file)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&cfg); err != nil {
		return config{}, fmt.Errorf("invalid Probst config JSON")
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return config{}, fmt.Errorf("invalid Probst config JSON (trailing data)")
	}
	return cfg, nil
}

func envOrFile(key, fromFile string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fromFile
}
