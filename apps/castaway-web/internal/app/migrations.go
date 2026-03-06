package app

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func RunMigrations(ctx context.Context, pool *pgxpool.Pool, migrationsDir string) error {
	if _, err := pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			name TEXT PRIMARY KEY,
			applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)
	`); err != nil {
		return fmt.Errorf("create schema_migrations table: %w", err)
	}

	entries, err := os.ReadDir(migrationsDir)
	if err != nil {
		return fmt.Errorf("read migrations dir: %w", err)
	}

	migrationFiles := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if filepath.Ext(entry.Name()) != ".sql" {
			continue
		}
		migrationFiles = append(migrationFiles, entry.Name())
	}
	sort.Strings(migrationFiles)

	for _, migrationFile := range migrationFiles {
		var applied bool
		if err := pool.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE name = $1)", migrationFile).Scan(&applied); err != nil {
			return fmt.Errorf("check migration %s: %w", migrationFile, err)
		}
		if applied {
			continue
		}

		migrationPath := filepath.Join(migrationsDir, migrationFile)
		contents, err := os.ReadFile(migrationPath)
		if err != nil {
			return fmt.Errorf("read migration %s: %w", migrationFile, err)
		}

		tx, err := pool.Begin(ctx)
		if err != nil {
			return fmt.Errorf("begin tx for migration %s: %w", migrationFile, err)
		}

		if _, err := tx.Exec(ctx, string(contents)); err != nil {
			if rollbackErr := tx.Rollback(ctx); rollbackErr != nil && !errors.Is(rollbackErr, pgx.ErrTxClosed) {
				return fmt.Errorf("rollback migration %s after apply error: %w", migrationFile, rollbackErr)
			}
			return fmt.Errorf("apply migration %s: %w", migrationFile, err)
		}
		if _, err := tx.Exec(ctx, "INSERT INTO schema_migrations (name) VALUES ($1)", migrationFile); err != nil {
			if rollbackErr := tx.Rollback(ctx); rollbackErr != nil && !errors.Is(rollbackErr, pgx.ErrTxClosed) {
				return fmt.Errorf("rollback migration %s after record error: %w", migrationFile, rollbackErr)
			}
			return fmt.Errorf("record migration %s: %w", migrationFile, err)
		}
		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("commit migration %s: %w", migrationFile, err)
		}
	}

	return nil
}

func PendingMigrations(ctx context.Context, pool *pgxpool.Pool, migrationsDir string) ([]string, error) {
	entries, err := os.ReadDir(migrationsDir)
	if err != nil {
		return nil, fmt.Errorf("read migrations dir: %w", err)
	}
	migrationFiles := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".sql" {
			continue
		}
		migrationFiles = append(migrationFiles, entry.Name())
	}
	sort.Strings(migrationFiles)

	pending := make([]string, 0)
	for _, file := range migrationFiles {
		var applied bool
		err := pool.QueryRow(ctx, `
			SELECT EXISTS(
				SELECT 1
				FROM information_schema.tables
				WHERE table_name = 'schema_migrations'
			)
		`).Scan(&applied)
		if err != nil {
			return nil, fmt.Errorf("check schema_migrations table: %w", err)
		}
		if !applied {
			return migrationFiles, nil
		}

		var migrationApplied bool
		if err := pool.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE name = $1)", file).Scan(&migrationApplied); err != nil {
			if strings.Contains(err.Error(), "schema_migrations") {
				return migrationFiles, nil
			}
			return nil, fmt.Errorf("check applied migration %s: %w", file, err)
		}
		if !migrationApplied {
			pending = append(pending, file)
		}
	}

	return pending, nil
}
