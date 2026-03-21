package storage

import (
	"fmt"
	"go-port-forward/internal/models"
	"go-port-forward/pkg/serializer/json"
	"time"

	bolt "go.etcd.io/bbolt"
)

var rulesBucket = []byte("rules")

// Store provides persistent storage for forwarding rules.
type Store interface {
	ListRules() ([]*models.ForwardRule, error)
	GetRule(id string) (*models.ForwardRule, error)
	SaveRule(rule *models.ForwardRule) error
	DeleteRule(id string) error
	Close() error
}

type boltStore struct {
	db *bolt.DB
}

// Open opens (or creates) the bbolt database at path.
func Open(path string) (Store, error) {
	db, err := bolt.Open(path, 0o600, &bolt.Options{Timeout: 2 * time.Second})
	if err != nil {
		return nil, fmt.Errorf("storage: open %s: %w", path, err)
	}
	// Ensure bucket exists
	if err = db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists(rulesBucket)
		return err
	}); err != nil {
		_ = db.Close()
		return nil, err
	}
	return &boltStore{db: db}, nil
}

func (s *boltStore) ListRules() ([]*models.ForwardRule, error) {
	var rules []*models.ForwardRule
	err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(rulesBucket)
		return b.ForEach(func(_, v []byte) error {
			var r models.ForwardRule
			if err := json.Unmarshal(v, &r); err != nil {
				return err
			}
			rules = append(rules, &r)
			return nil
		})
	})
	return rules, err
}

func (s *boltStore) GetRule(id string) (*models.ForwardRule, error) {
	var rule models.ForwardRule
	err := s.db.View(func(tx *bolt.Tx) error {
		v := tx.Bucket(rulesBucket).Get([]byte(id))
		if v == nil {
			return fmt.Errorf("rule %q not found", id)
		}
		return json.Unmarshal(v, &rule)
	})
	if err != nil {
		return nil, err
	}
	return &rule, nil
}

func (s *boltStore) SaveRule(rule *models.ForwardRule) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		data, err := json.Marshal(rule)
		if err != nil {
			return err
		}
		return tx.Bucket(rulesBucket).Put([]byte(rule.ID), data)
	})
}

func (s *boltStore) DeleteRule(id string) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		return tx.Bucket(rulesBucket).Delete([]byte(id))
	})
}

func (s *boltStore) Close() error { return s.db.Close() }
