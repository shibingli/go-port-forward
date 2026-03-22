package forward

import (
	"errors"
	"net"
	"path/filepath"
	"strconv"
	"testing"

	"go-port-forward/internal/config"
	"go-port-forward/internal/logger"
	"go-port-forward/internal/models"
	"go-port-forward/internal/storage"
	"go.uber.org/zap"
)

func TestUpdateRuleConflictDoesNotStopExistingForwarder(t *testing.T) {
	logger.L = zap.NewNop()
	logger.S = logger.L.Sugar()

	store, err := storage.Open(filepath.Join(t.TempDir(), "rules.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer store.Close()

	mgr, err := NewManager(store, config.ForwardConfig{DialTimeout: 1, UDPTimeout: 30, BufferSize: 4096, PoolSize: 32})
	if err != nil {
		t.Fatalf("new manager: %v", err)
	}
	defer mgr.Shutdown()

	listenPort := freeTCPPort(t)
	conflictPort := freeTCPPort(t)
	targetPort := freeTCPPort(t)

	rule, err := mgr.AddRule(&models.CreateRuleRequest{
		Name:       "active-rule",
		ListenAddr: "127.0.0.1",
		ListenPort: listenPort,
		Protocol:   models.ProtocolTCP,
		TargetAddr: "127.0.0.1",
		TargetPort: targetPort,
		Enabled:    true,
	})
	if err != nil {
		t.Fatalf("add active rule: %v", err)
	}

	_, err = mgr.AddRule(&models.CreateRuleRequest{
		Name:       "conflict-rule",
		ListenAddr: "127.0.0.1",
		ListenPort: conflictPort,
		Protocol:   models.ProtocolTCP,
		TargetAddr: "127.0.0.1",
		TargetPort: targetPort,
		Enabled:    false,
	})
	if err != nil {
		t.Fatalf("add conflict rule: %v", err)
	}

	assertTCPDialSucceeds(t, listenPort)

	_, err = mgr.UpdateRule(rule.ID, &models.UpdateRuleRequest{ListenPort: &conflictPort})
	if !errors.Is(err, ErrPortConflict) {
		t.Fatalf("expected ErrPortConflict, got %v", err)
	}

	assertTCPDialSucceeds(t, listenPort)

	stored, err := mgr.GetRule(rule.ID)
	if err != nil {
		t.Fatalf("get rule: %v", err)
	}
	if stored.ListenPort != listenPort {
		t.Fatalf("listen port changed after failed update: got %d want %d", stored.ListenPort, listenPort)
	}
}

func freeTCPPort(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserve port: %v", err)
	}
	defer ln.Close()
	return ln.Addr().(*net.TCPAddr).Port
}

func assertTCPDialSucceeds(t *testing.T, port int) {
	t.Helper()
	conn, err := net.Dial("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)))
	if err != nil {
		t.Fatalf("dial listener on %d: %v", port, err)
	}
	_ = conn.Close()
}
