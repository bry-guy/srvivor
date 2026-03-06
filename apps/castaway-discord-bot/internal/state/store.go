package state

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	bolt "go.etcd.io/bbolt"
)

const (
	guildDefaultsBucket = "guild_defaults"
	userDefaultsBucket  = "user_defaults"
)

type Store struct {
	db *bolt.DB
}

func Open(path string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("create state dir: %w", err)
	}

	db, err := bolt.Open(path, 0o600, &bolt.Options{Timeout: time.Second})
	if err != nil {
		return nil, fmt.Errorf("open state db: %w", err)
	}

	store := &Store{db: db}
	if err := store.db.Update(func(tx *bolt.Tx) error {
		if _, err := tx.CreateBucketIfNotExists([]byte(guildDefaultsBucket)); err != nil {
			return err
		}
		if _, err := tx.CreateBucketIfNotExists([]byte(userDefaultsBucket)); err != nil {
			return err
		}
		return nil
	}); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("initialize state db: %w", err)
	}

	return store, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) SetGuildDefault(guildID, instanceID string) error {
	return s.put(guildDefaultsBucket, strings.TrimSpace(guildID), strings.TrimSpace(instanceID))
}

func (s *Store) GetGuildDefault(guildID string) (string, error) {
	return s.get(guildDefaultsBucket, strings.TrimSpace(guildID))
}

func (s *Store) ClearGuildDefault(guildID string) error {
	return s.delete(guildDefaultsBucket, strings.TrimSpace(guildID))
}

func (s *Store) SetUserDefault(guildID, userID, instanceID string) error {
	return s.put(userDefaultsBucket, userKey(guildID, userID), strings.TrimSpace(instanceID))
}

func (s *Store) GetUserDefault(guildID, userID string) (string, error) {
	return s.get(userDefaultsBucket, userKey(guildID, userID))
}

func (s *Store) ClearUserDefault(guildID, userID string) error {
	return s.delete(userDefaultsBucket, userKey(guildID, userID))
}

func (s *Store) put(bucketName, key, value string) error {
	if key == "" {
		return fmt.Errorf("state key is required")
	}
	if value == "" {
		return fmt.Errorf("state value is required")
	}
	return s.db.Update(func(tx *bolt.Tx) error {
		return tx.Bucket([]byte(bucketName)).Put([]byte(key), []byte(value))
	})
}

func (s *Store) get(bucketName, key string) (string, error) {
	if key == "" {
		return "", nil
	}
	var value string
	err := s.db.View(func(tx *bolt.Tx) error {
		data := tx.Bucket([]byte(bucketName)).Get([]byte(key))
		if len(data) == 0 {
			return nil
		}
		value = string(data)
		return nil
	})
	if err != nil {
		return "", err
	}
	return value, nil
}

func (s *Store) delete(bucketName, key string) error {
	if key == "" {
		return nil
	}
	return s.db.Update(func(tx *bolt.Tx) error {
		return tx.Bucket([]byte(bucketName)).Delete([]byte(key))
	})
}

func userKey(guildID, userID string) string {
	return strings.TrimSpace(guildID) + ":" + strings.TrimSpace(userID)
}
