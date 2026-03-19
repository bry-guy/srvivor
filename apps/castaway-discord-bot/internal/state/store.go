package state

import "strings"

type Backend string

const (
	BackendBolt     Backend = "bolt"
	BackendPostgres Backend = "postgres"
)

type Store interface {
	Close() error
	SetGuildDefault(guildID, instanceID string) error
	GetGuildDefault(guildID string) (string, error)
	ClearGuildDefault(guildID string) error
	SetUserDefault(guildID, userID, instanceID string) error
	GetUserDefault(guildID, userID string) (string, error)
	ClearUserDefault(guildID, userID string) error
}

type Options struct {
	Backend     Backend
	BoltPath    string
	PostgresURL string
	AutoMigrate bool
}

func userKey(guildID, userID string) string {
	return strings.TrimSpace(guildID) + ":" + strings.TrimSpace(userID)
}

func trimOrEmpty(value string) string {
	return strings.TrimSpace(value)
}
