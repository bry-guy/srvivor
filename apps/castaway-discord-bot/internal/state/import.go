package state

import (
	"context"
	"fmt"
	"strings"

	bolt "go.etcd.io/bbolt"
)

type ImportResult struct {
	GuildDefaultsImported int
	UserDefaultsImported  int
}

func ImportBoltToPostgres(ctx context.Context, boltPath, postgresURL string) (*ImportResult, error) {
	source, err := OpenBolt(boltPath)
	if err != nil {
		return nil, fmt.Errorf("open bolt source store: %w", err)
	}
	defer source.Close()

	target, err := OpenPostgres(ctx, postgresURL, true)
	if err != nil {
		return nil, fmt.Errorf("open postgres target store: %w", err)
	}
	defer target.Close()

	result := &ImportResult{}

	if err := source.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(guildDefaultsBucket))
		if bucket == nil {
			return nil
		}
		return bucket.ForEach(func(k, v []byte) error {
			if err := target.SetGuildDefault(string(k), string(v)); err != nil {
				return err
			}
			result.GuildDefaultsImported++
			return nil
		})
	}); err != nil {
		return nil, fmt.Errorf("import guild defaults: %w", err)
	}

	if err := source.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(userDefaultsBucket))
		if bucket == nil {
			return nil
		}
		return bucket.ForEach(func(k, v []byte) error {
			guildID, userID, ok := splitUserKey(string(k))
			if !ok {
				return fmt.Errorf("invalid bolt user default key: %q", string(k))
			}
			if err := target.SetUserDefault(guildID, userID, string(v)); err != nil {
				return err
			}
			result.UserDefaultsImported++
			return nil
		})
	}); err != nil {
		return nil, fmt.Errorf("import user defaults: %w", err)
	}

	return result, nil
}

func splitUserKey(value string) (string, string, bool) {
	parts := strings.SplitN(value, ":", 2)
	if len(parts) != 2 {
		return "", "", false
	}
	guildID := trimOrEmpty(parts[0])
	userID := trimOrEmpty(parts[1])
	if guildID == "" || userID == "" {
		return "", "", false
	}
	return guildID, userID, true
}
