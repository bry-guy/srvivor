package state

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type postgresQuerier interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Close()
}

type PostgresStore struct {
	pool postgresQuerier
}

func OpenPostgres(ctx context.Context, databaseURL string, autoMigrate bool) (*PostgresStore, error) {
	databaseURL = strings.TrimSpace(databaseURL)
	if databaseURL == "" {
		return nil, fmt.Errorf("postgres database url is required")
	}

	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, fmt.Errorf("connect postgres state db: %w", err)
	}

	store := &PostgresStore{pool: pool}
	if autoMigrate {
		if err := store.ensureSchema(ctx); err != nil {
			pool.Close()
			return nil, err
		}
	}

	return store, nil
}

func (s *PostgresStore) Close() error {
	s.pool.Close()
	return nil
}

func (s *PostgresStore) SetGuildDefault(guildID, instanceID string) error {
	guildID = trimOrEmpty(guildID)
	instanceID = trimOrEmpty(instanceID)
	if guildID == "" {
		return fmt.Errorf("state key is required")
	}
	if instanceID == "" {
		return fmt.Errorf("state value is required")
	}

	_, err := s.pool.Exec(context.Background(), `
		INSERT INTO guild_defaults (guild_id, instance_id)
		VALUES ($1, $2)
		ON CONFLICT (guild_id)
		DO UPDATE SET instance_id = EXCLUDED.instance_id, updated_at = NOW()
	`, guildID, instanceID)
	if err != nil {
		return fmt.Errorf("upsert guild default: %w", err)
	}
	return nil
}

func (s *PostgresStore) GetGuildDefault(guildID string) (string, error) {
	guildID = trimOrEmpty(guildID)
	if guildID == "" {
		return "", nil
	}

	var instanceID string
	err := s.pool.QueryRow(context.Background(), `
		SELECT instance_id
		FROM guild_defaults
		WHERE guild_id = $1
	`, guildID).Scan(&instanceID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", nil
		}
		return "", fmt.Errorf("get guild default: %w", err)
	}
	return instanceID, nil
}

func (s *PostgresStore) ClearGuildDefault(guildID string) error {
	guildID = trimOrEmpty(guildID)
	if guildID == "" {
		return nil
	}
	_, err := s.pool.Exec(context.Background(), `DELETE FROM guild_defaults WHERE guild_id = $1`, guildID)
	if err != nil {
		return fmt.Errorf("clear guild default: %w", err)
	}
	return nil
}

func (s *PostgresStore) SetUserDefault(guildID, userID, instanceID string) error {
	guildID = trimOrEmpty(guildID)
	userID = trimOrEmpty(userID)
	instanceID = trimOrEmpty(instanceID)
	if guildID == "" || userID == "" {
		return fmt.Errorf("state key is required")
	}
	if instanceID == "" {
		return fmt.Errorf("state value is required")
	}

	_, err := s.pool.Exec(context.Background(), `
		INSERT INTO user_defaults (guild_id, user_id, instance_id)
		VALUES ($1, $2, $3)
		ON CONFLICT (guild_id, user_id)
		DO UPDATE SET instance_id = EXCLUDED.instance_id, updated_at = NOW()
	`, guildID, userID, instanceID)
	if err != nil {
		return fmt.Errorf("upsert user default: %w", err)
	}
	return nil
}

func (s *PostgresStore) GetUserDefault(guildID, userID string) (string, error) {
	guildID = trimOrEmpty(guildID)
	userID = trimOrEmpty(userID)
	if guildID == "" || userID == "" {
		return "", nil
	}

	var instanceID string
	err := s.pool.QueryRow(context.Background(), `
		SELECT instance_id
		FROM user_defaults
		WHERE guild_id = $1 AND user_id = $2
	`, guildID, userID).Scan(&instanceID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", nil
		}
		return "", fmt.Errorf("get user default: %w", err)
	}
	return instanceID, nil
}

func (s *PostgresStore) ClearUserDefault(guildID, userID string) error {
	guildID = trimOrEmpty(guildID)
	userID = trimOrEmpty(userID)
	if guildID == "" || userID == "" {
		return nil
	}
	_, err := s.pool.Exec(context.Background(), `DELETE FROM user_defaults WHERE guild_id = $1 AND user_id = $2`, guildID, userID)
	if err != nil {
		return fmt.Errorf("clear user default: %w", err)
	}
	return nil
}

func (s *PostgresStore) ensureSchema(ctx context.Context) error {
	for _, stmt := range []string{
		`CREATE TABLE IF NOT EXISTS guild_defaults (
			guild_id TEXT PRIMARY KEY,
			instance_id TEXT NOT NULL,
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`,
		`CREATE TABLE IF NOT EXISTS user_defaults (
			guild_id TEXT NOT NULL,
			user_id TEXT NOT NULL,
			instance_id TEXT NOT NULL,
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			PRIMARY KEY (guild_id, user_id)
		)`,
	} {
		if _, err := s.pool.Exec(ctx, stmt); err != nil {
			return fmt.Errorf("ensure postgres state schema: %w", err)
		}
	}
	return nil
}
