package storage

import (
	"path/filepath"
	"testing"
	"time"

	"go-port-forward/internal/models"
	"go-port-forward/pkg/serializer/json"
	bolt "go.etcd.io/bbolt"
)

func TestSaveRuleScrubsRuntimeFields(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "rules.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer store.Close()

	rule := &models.ForwardRule{
		ID:          "rule-1",
		Name:        "test",
		Protocol:    models.ProtocolTCP,
		ListenAddr:  "127.0.0.1",
		ListenPort:  8080,
		TargetAddr:  "127.0.0.1",
		TargetPort:  9090,
		Enabled:     true,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
		Status:      models.StatusActive,
		ErrorMsg:    "boom",
		BytesIn:     123,
		BytesOut:    456,
		ActiveConns: 7,
		TotalConns:  8,
	}
	if err := store.SaveRule(rule); err != nil {
		t.Fatalf("save rule: %v", err)
	}

	bs := store.(*boltStore)
	var persisted models.ForwardRule
	if err := bs.db.View(func(tx *bolt.Tx) error {
		data := tx.Bucket(rulesBucket).Get([]byte(rule.ID))
		return json.Unmarshal(data, &persisted)
	}); err != nil {
		t.Fatalf("read raw rule: %v", err)
	}

	assertTransientStateScrubbed(t, &persisted)
	assertTrafficStatePreserved(t, &persisted, rule)

	loaded, err := store.GetRule(rule.ID)
	if err != nil {
		t.Fatalf("get rule: %v", err)
	}
	assertTransientStateScrubbed(t, loaded)
	assertTrafficStatePreserved(t, loaded, rule)

	list, err := store.ListRules()
	if err != nil {
		t.Fatalf("list rules: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("list length = %d, want 1", len(list))
	}
	assertTransientStateScrubbed(t, list[0])
	assertTrafficStatePreserved(t, list[0], rule)
}

func assertTransientStateScrubbed(t *testing.T, rule *models.ForwardRule) {
	t.Helper()
	if rule.Status != "" || rule.ErrorMsg != "" {
		t.Fatalf("transient runtime fields should be scrubbed: %+v", rule)
	}
}

func assertTrafficStatePreserved(t *testing.T, got, want *models.ForwardRule) {
	t.Helper()
	if got.BytesIn != want.BytesIn || got.BytesOut != want.BytesOut || got.ActiveConns != want.ActiveConns || got.TotalConns != want.TotalConns {
		t.Fatalf("traffic counters changed unexpectedly: got=%+v want=%+v", got, want)
	}
}
