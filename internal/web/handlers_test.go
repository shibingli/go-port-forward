package web

import (
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"go-port-forward/internal/config"
	"go-port-forward/internal/firewall"
	"go-port-forward/internal/forward"
	"go-port-forward/internal/logger"
	"go-port-forward/internal/models"
	"go-port-forward/internal/storage"
	"go-port-forward/pkg/os/wsl"
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

func TestToggleRuleSyncsFirewallOnEnableAndDisable(t *testing.T) {
	fw := &fakeFirewall{}
	h, cleanup := newTestHandlerWithFirewall(t, fw)
	defer cleanup()

	rule, err := h.mgr.AddRule(&models.CreateRuleRequest{
		Name:        "toggle-fw",
		ListenAddr:  "127.0.0.1",
		ListenPort:  freePort(t),
		Protocol:    models.ProtocolTCP,
		TargetAddr:  "127.0.0.1",
		TargetPort:  freePort(t),
		Enabled:     false,
		AddFirewall: true,
	})
	if err != nil {
		t.Fatalf("seed rule: %v", err)
	}

	enableReq := httptest.NewRequest(http.MethodPut, "/api/rules/"+rule.ID+"/toggle", strings.NewReader(`{"enabled":true}`))
	enableReq.SetPathValue("id", rule.ID)
	enableRec := httptest.NewRecorder()
	h.toggleRule(enableRec, enableReq)
	if enableRec.Code != http.StatusOK {
		t.Fatalf("enable status = %d, want 200, body=%s", enableRec.Code, enableRec.Body.String())
	}
	if len(fw.added) != 1 {
		t.Fatalf("firewall add calls = %d, want 1", len(fw.added))
	}

	disableReq := httptest.NewRequest(http.MethodPut, "/api/rules/"+rule.ID+"/toggle", strings.NewReader(`{"enabled":false}`))
	disableReq.SetPathValue("id", rule.ID)
	disableRec := httptest.NewRecorder()
	h.toggleRule(disableRec, disableReq)
	if disableRec.Code != http.StatusOK {
		t.Fatalf("disable status = %d, want 200, body=%s", disableRec.Code, disableRec.Body.String())
	}
	if len(fw.deleted) != 1 {
		t.Fatalf("firewall delete calls = %d, want 1", len(fw.deleted))
	}
}

func TestUpdateRuleSyncsFirewallWhenEndpointChanges(t *testing.T) {
	fw := &fakeFirewall{}
	h, cleanup := newTestHandlerWithFirewall(t, fw)
	defer cleanup()

	rule, err := h.mgr.AddRule(&models.CreateRuleRequest{
		Name:        "fw-update",
		ListenAddr:  "127.0.0.1",
		ListenPort:  freePort(t),
		Protocol:    models.ProtocolTCP,
		TargetAddr:  "127.0.0.1",
		TargetPort:  freePort(t),
		Enabled:     true,
		AddFirewall: true,
	})
	if err != nil {
		t.Fatalf("seed rule: %v", err)
	}

	newPort := freePort(t)
	req := httptest.NewRequest(http.MethodPut, "/api/rules/"+rule.ID, strings.NewReader(`{"listen_port":`+strconv.Itoa(newPort)+`}`))
	req.SetPathValue("id", rule.ID)
	rec := httptest.NewRecorder()

	h.updateRule(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body=%s", rec.Code, rec.Body.String())
	}
	if len(fw.deleted) != 1 || fw.deleted[0].Port != rule.ListenPort {
		t.Fatalf("unexpected firewall delete calls: %#v", fw.deleted)
	}
	if len(fw.added) != 1 || fw.added[0].Port != newPort {
		t.Fatalf("unexpected firewall add calls: %#v", fw.added)
	}
}

func TestWSLCapabilityEndpointReturnsCapabilityPayload(t *testing.T) {
	h, cleanup := newTestHandler(t)
	defer cleanup()

	oldDetect := wslDetectCapability
	wslDetectCapability = func() wsl.Capability {
		return wsl.Capability{
			Supported:  true,
			Installed:  true,
			Enabled:    true,
			HasDistros: true,
			ShowImport: true,
			Distros:    []wsl.Distro{{Name: "Ubuntu-24.04", State: "Running", Version: "2", Default: true}},
		}
	}
	defer func() { wslDetectCapability = oldDetect }()

	req := httptest.NewRequest(http.MethodGet, "/api/wsl/capability", nil)
	rec := httptest.NewRecorder()

	h.wslCapability(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body=%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, `"show_import":true`) || !strings.Contains(body, `Ubuntu-24.04`) {
		t.Fatalf("unexpected capability payload: %s", body)
	}
}

func TestDashboardReturnsRulesAndStatsInSinglePayload(t *testing.T) {
	h, cleanup := newTestHandler(t)
	defer cleanup()

	_, err := h.mgr.AddRule(&models.CreateRuleRequest{
		Name:       "dashboard-rule",
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

	req := httptest.NewRequest(http.MethodGet, "/api/dashboard", nil)
	rec := httptest.NewRecorder()

	h.dashboard(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body=%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, `"rules"`) || !strings.Contains(body, `"stats"`) || !strings.Contains(body, `dashboard-rule`) {
		t.Fatalf("unexpected dashboard payload: %s", body)
	}
}

func TestDiagnosticsReturnsRuntimePoolAndManagerSnapshot(t *testing.T) {
	h, cleanup := newTestHandler(t)
	defer cleanup()

	_, err := h.mgr.AddRule(&models.CreateRuleRequest{
		Name:       "diag-rule",
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
	occupied, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("occupy listen port: %v", err)
	}
	defer occupied.Close()
	_, err = h.mgr.AddRule(&models.CreateRuleRequest{
		Name:       "diag-error",
		ListenAddr: "127.0.0.1",
		ListenPort: occupied.Addr().(*net.TCPAddr).Port,
		Protocol:   models.ProtocolBoth,
		TargetAddr: "127.0.0.1",
		TargetPort: freePort(t),
		Enabled:    true,
	})
	if err != nil {
		t.Fatalf("seed error rule: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/diagnostics", nil)
	rec := httptest.NewRecorder()

	h.diagnostics(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body=%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	for _, needle := range []string{`"runtime"`, `"pool"`, `"manager"`, `"protocols"`, `"hot_rules"`, `"top_active_rules"`, `"top_traffic_rules"`, `"top_error_rules"`, `"last_error_at"`, `"last_status_change_at"`, `"error_count"`, `"errors"`, `"cached_rules":2`, `"inactive":1`, `"error":1`, `diag-error`} {
		if !strings.Contains(body, needle) {
			t.Fatalf("diagnostics payload missing %s: %s", needle, body)
		}
	}
}

func newTestHandler(t *testing.T) (*handler, func()) {
	return newTestHandlerWithFirewall(t, nil)
}

func newTestHandlerWithFirewall(t *testing.T, fw firewall.Manager) (*handler, func()) {
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
	return &handler{mgr: mgr, fw: fw}, func() {
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

type fakeFirewall struct {
	added   []firewall.Rule
	deleted []firewall.Rule
}

func (f *fakeFirewall) AddRule(r firewall.Rule) error {
	f.added = append(f.added, r)
	return nil
}

func (f *fakeFirewall) DeleteRule(r firewall.Rule) error {
	f.deleted = append(f.deleted, r)
	return nil
}

func (f *fakeFirewall) RuleExists(firewall.Rule) (bool, error) {
	return false, nil
}
