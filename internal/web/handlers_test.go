package web

import (
	"net"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"go-port-forward/internal/config"
	"go-port-forward/internal/forward"
	"go-port-forward/internal/logger"
	"go-port-forward/internal/models"
	"go-port-forward/internal/storage"
	"go.uber.org/zap"
)

func TestCreateRuleRejectsUnknownJSONFields(t *testing.T) {
	h, cleanup := newTestHandler(t)
	defer cleanup()

	req := httptest.NewRequest("POST", "/api/rules", strings.NewReader(`{"name":"demo","listen_port":18080,"protocol":"tcp","target_addr":"127.0.0.1","target_port":80,"extra":true}`))
	rec := httptest.NewRecorder()

	h.createRule(rec, req)

	if rec.Code != 400 {
		t.Fatalf("status = %d, want 400, body=%s", rec.Code, rec.Body.String())
	}
}

func TestCreateRuleMapsConflictTo409(t *testing.T) {
	h, cleanup := newTestHandler(t)
	defer cleanup()

	port := freePort(t)
	_, err := h.mgr.AddRule(&models.CreateRuleRequest{
		Name:       "existing",
		ListenAddr: "127.0.0.1",
		ListenPort: port,
		Protocol:   models.ProtocolTCP,
		TargetAddr: "127.0.0.1",
		TargetPort: freePort(t),
		Enabled:    false,
	})
	if err != nil {
		t.Fatalf("seed rule: %v", err)
	}

	req := httptest.NewRequest("POST", "/api/rules", strings.NewReader(`{"name":"dup","listen_addr":"127.0.0.1","listen_port":`+strconv.Itoa(port)+`,"protocol":"tcp","target_addr":"127.0.0.1","target_port":8080}`))
	rec := httptest.NewRecorder()

	h.createRule(rec, req)

	if rec.Code != 409 {
		t.Fatalf("status = %d, want 409, body=%s", rec.Code, rec.Body.String())
	}
}

func TestToggleRuleRequiresEnabledField(t *testing.T) {
	h, cleanup := newTestHandler(t)
	defer cleanup()

	rule, err := h.mgr.AddRule(&models.CreateRuleRequest{
		Name:       "toggle-me",
		ListenAddr: "127.0.0.1",
		ListenPort: freePort(t),
		Protocol:   models.ProtocolTCP,
		TargetAddr: "127.0.0.1",
		TargetPort: freePort(t),
		Enabled:    false,
	})
	if err != nil {
		t.Fatalf("seed rule: %v", err)
	}

	req := httptest.NewRequest("PUT", "/api/rules/"+rule.ID+"/toggle", strings.NewReader(`{}`))
	req.SetPathValue("id", rule.ID)
	rec := httptest.NewRecorder()

	h.toggleRule(rec, req)

	if rec.Code != 400 {
		t.Fatalf("status = %d, want 400, body=%s", rec.Code, rec.Body.String())
	}
}

func TestServerStartReturnsErrorWhenPortIsOccupied(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()

	port := ln.Addr().(*net.TCPAddr).Port
	srv := New(config.WebConfig{Host: "127.0.0.1", Port: port}, nil, nil)
	if err := srv.Start(); err == nil {
		t.Fatal("expected Start to fail when port is occupied")
	}
}

func newTestHandler(t *testing.T) (*handler, func()) {
	t.Helper()
	logger.L = zap.NewNop()
	logger.S = logger.L.Sugar()

	store, err := storage.Open(filepath.Join(t.TempDir(), "rules.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	mgr, err := forward.NewManager(store, config.ForwardConfig{DialTimeout: 1, UDPTimeout: 30, BufferSize: 4096, PoolSize: 32})
	if err != nil {
		_ = store.Close()
		t.Fatalf("new manager: %v", err)
	}
	return &handler{mgr: mgr}, func() {
		mgr.Shutdown()
		_ = store.Close()
	}
}

func freePort(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserve port: %v", err)
	}
	defer ln.Close()
	return ln.Addr().(*net.TCPAddr).Port
}
