package state

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	bolt "go.etcd.io/bbolt"
)

const (
	guildDefaultsBucket = "guild_defaults"
	userDefaultsBucket  = "user_defaults"
)

type BoltStore struct {
	db *bolt.DB
}

func OpenBolt(path string) (*BoltStore, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("create state dir: %w", err)
	}

	db, err := bolt.Open(path, 0o600, &bolt.Options{Timeout: time.Second})
	if err != nil {
		return nil, fmt.Errorf("open state db: %w", err)
	}

	store := &BoltStore{db: db}
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

func (s *BoltStore) Close() error {
	return s.db.Close()
}

func (s *BoltStore) SetGuildDefault(guildID, instanceID string) error {
	return s.put(guildDefaultsBucket, trimOrEmpty(guildID), trimOrEmpty(instanceID))
}

func (s *BoltStore) GetGuildDefault(guildID string) (string, error) {
	return s.get(guildDefaultsBucket, trimOrEmpty(guildID))
}

func (s *BoltStore) ClearGuildDefault(guildID string) error {
	return s.delete(guildDefaultsBucket, trimOrEmpty(guildID))
}

func (s *BoltStore) SetUserDefault(guildID, userID, instanceID string) error {
	return s.put(userDefaultsBucket, userKey(guildID, userID), trimOrEmpty(instanceID))
}

func (s *BoltStore) GetUserDefault(guildID, userID string) (string, error) {
	return s.get(userDefaultsBucket, userKey(guildID, userID))
}

func (s *BoltStore) ClearUserDefault(guildID, userID string) error {
	return s.delete(userDefaultsBucket, userKey(guildID, userID))
}

func (s *BoltStore) put(bucketName, key, value string) error {
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

func (s *BoltStore) get(bucketName, key string) (string, error) {
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

func (s *BoltStore) delete(bucketName, key string) error {
	if key == "" {
		return nil
	}
	return s.db.Update(func(tx *bolt.Tx) error {
		return tx.Bucket([]byte(bucketName)).Delete([]byte(key))
	})
}
