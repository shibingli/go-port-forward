package forward

import (
	"fmt"
	"sync"
	"time"

	"go-port-forward/internal/config"
	"go-port-forward/internal/logger"
	"go-port-forward/internal/models"
	"go-port-forward/internal/storage"
	"go-port-forward/pkg/pool"

	"github.com/google/uuid"
)

type entry struct {
	tcp *TCPForwarder
	udp *UDPForwarder
}

// Manager owns the lifecycle of all active forwarders.
type Manager struct {
	store  storage.Store
	cfg    config.ForwardConfig
	mu     sync.RWMutex
	active map[string]*entry // rule ID → forwarders
	errors map[string]string // rule ID → last error message
}

// NewManager creates a Manager and loads existing rules from storage.
// The goroutine pool is managed globally via pkg/pool.
func NewManager(store storage.Store, cfg config.ForwardConfig) (*Manager, error) {
	// Ensure global goroutine pool is initialized (lazy init if not done yet).
	poolSize := cfg.PoolSize
	if poolSize <= 0 {
		poolSize = 10000
	}
	_ = pool.InitGoroutinePool(poolSize, true)

	m := &Manager{
		store:  store,
		cfg:    cfg,
		active: make(map[string]*entry),
		errors: make(map[string]string),
	}

	// Start all enabled rules persisted from a previous run.
	rules, err := store.ListRules()
	if err != nil {
		return nil, err
	}
	for _, r := range rules {
		if r.Enabled {
			if e2 := m.startForwarders(r); e2 != nil {
				logger.S.Warnw("failed to start rule on boot", "rule", r.Name, "err", e2)
				r.Status = models.StatusError
				m.errors[r.ID] = e2.Error()
			} else {
				r.Status = models.StatusActive
			}
		}
	}
	return m, nil
}

// AddRule validates, persists and starts a new rule.
func (m *Manager) AddRule(req *models.CreateRuleRequest) (*models.ForwardRule, error) {
	// Port conflict detection
	if err := m.checkPortConflict(req.ListenAddr, req.ListenPort, req.Protocol, ""); err != nil {
		return nil, err
	}
	r := &models.ForwardRule{
		ID:          uuid.NewString(),
		Name:        req.Name,
		ListenAddr:  req.ListenAddr,
		ListenPort:  req.ListenPort,
		Protocol:    req.Protocol,
		TargetAddr:  req.TargetAddr,
		TargetPort:  req.TargetPort,
		AddFirewall: req.AddFirewall,
		Comment:     req.Comment,
		Enabled:     req.Enabled,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	if err := m.store.SaveRule(r); err != nil {
		return nil, err
	}
	if r.Enabled {
		if err := m.startForwarders(r); err != nil {
			r.Status = models.StatusError
			r.ErrorMsg = err.Error()
			m.mu.Lock()
			m.errors[r.ID] = err.Error()
			m.mu.Unlock()
		} else {
			r.Status = models.StatusActive
		}
	} else {
		r.Status = models.StatusInactive
	}
	return r, nil
}

// UpdateRule applies partial updates to an existing rule (restarts forwarders as needed).
func (m *Manager) UpdateRule(id string, req *models.UpdateRuleRequest) (*models.ForwardRule, error) {
	r, err := m.store.GetRule(id)
	if err != nil {
		return nil, err
	}
	m.stopForwarders(id)

	if req.Name != nil {
		r.Name = *req.Name
	}
	if req.ListenAddr != nil {
		r.ListenAddr = *req.ListenAddr
	}
	if req.ListenPort != nil {
		r.ListenPort = *req.ListenPort
	}
	if req.Protocol != nil {
		r.Protocol = *req.Protocol
	}
	if req.TargetAddr != nil {
		r.TargetAddr = *req.TargetAddr
	}
	if req.TargetPort != nil {
		r.TargetPort = *req.TargetPort
	}
	if req.AddFirewall != nil {
		r.AddFirewall = *req.AddFirewall
	}
	if req.Comment != nil {
		r.Comment = *req.Comment
	}
	if req.Enabled != nil {
		r.Enabled = *req.Enabled
	}
	r.UpdatedAt = time.Now()

	// Port conflict detection (exclude self)
	if err := m.checkPortConflict(r.ListenAddr, r.ListenPort, r.Protocol, id); err != nil {
		return nil, err
	}

	if err := m.store.SaveRule(r); err != nil {
		return nil, err
	}
	if r.Enabled {
		if err := m.startForwarders(r); err != nil {
			r.Status = models.StatusError
			r.ErrorMsg = err.Error()
			m.mu.Lock()
			m.errors[r.ID] = err.Error()
			m.mu.Unlock()
		} else {
			r.Status = models.StatusActive
			m.mu.Lock()
			delete(m.errors, r.ID)
			m.mu.Unlock()
		}
	} else {
		r.Status = models.StatusInactive
		m.mu.Lock()
		delete(m.errors, r.ID)
		m.mu.Unlock()
	}
	return r, nil
}

// DeleteRule stops and removes a rule permanently.
func (m *Manager) DeleteRule(id string) error {
	m.stopForwarders(id)
	m.mu.Lock()
	delete(m.errors, id)
	m.mu.Unlock()
	return m.store.DeleteRule(id)
}

// ToggleRule enables or disables a rule.
func (m *Manager) ToggleRule(id string, enabled bool) (*models.ForwardRule, error) {
	on := enabled
	return m.UpdateRule(id, &models.UpdateRuleRequest{Enabled: &on})
}

// ListRules returns all rules with live stats merged in.
func (m *Manager) ListRules() ([]*models.ForwardRule, error) {
	rules, err := m.store.ListRules()
	if err != nil {
		return nil, err
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, r := range rules {
		if e, ok := m.active[r.ID]; ok {
			r.Status = models.StatusActive
			r.BytesIn, r.BytesOut, r.ActiveConns, r.TotalConns = mergeStats(e)
		} else if r.Enabled {
			r.Status = models.StatusError
			r.ErrorMsg = m.errors[r.ID]
		} else {
			r.Status = models.StatusInactive
		}
	}
	return rules, nil
}

// GetRule returns one rule with live stats.
func (m *Manager) GetRule(id string) (*models.ForwardRule, error) {
	r, err := m.store.GetRule(id)
	if err != nil {
		return nil, err
	}
	m.mu.RLock()
	if e, ok := m.active[id]; ok {
		r.Status = models.StatusActive
		r.BytesIn, r.BytesOut, r.ActiveConns, r.TotalConns = mergeStats(e)
	} else if r.Enabled {
		r.Status = models.StatusError
		r.ErrorMsg = m.errors[id]
	} else {
		r.Status = models.StatusInactive
	}
	m.mu.RUnlock()
	return r, nil
}

// GlobalStats aggregates stats across all rules.
func (m *Manager) GlobalStats() *models.Stats {
	rules, _ := m.ListRules()
	s := &models.Stats{TotalRules: len(rules)}
	for _, r := range rules {
		if r.Enabled {
			s.EnabledRules++
		}
		if r.Status == models.StatusActive {
			s.ActiveRules++
		}
		s.TotalBytesIn += r.BytesIn
		s.TotalBytesOut += r.BytesOut
		s.TotalConns += r.TotalConns
	}
	return s
}

// Shutdown stops all active forwarders.
// The global goroutine pool is released separately in main shutdown.
func (m *Manager) Shutdown() {
	m.mu.Lock()
	for id := range m.active {
		m.stopForwardersLocked(id)
	}
	m.mu.Unlock()
}

// --- internal helpers ---

// checkPortConflict returns an error if the given listen addr/port/protocol
// overlaps with any existing rule (excluding the rule with excludeID).
func (m *Manager) checkPortConflict(listenAddr string, listenPort int, proto models.Protocol, excludeID string) error {
	rules, err := m.store.ListRules()
	if err != nil {
		return err
	}
	addrA := listenAddr
	if addrA == "" {
		addrA = "0.0.0.0"
	}
	for _, r := range rules {
		if r.ID == excludeID {
			continue
		}
		addrB := r.ListenAddr
		if addrB == "" {
			addrB = "0.0.0.0"
		}
		if addrA != addrB || r.ListenPort != listenPort {
			continue
		}
		if protocolsOverlap(proto, r.Protocol) {
			return fmt.Errorf("端口冲突 | Port conflict: %s:%d 已被规则 | already used by rule %q 占用 (协议 | protocol %s)",
				addrA, listenPort, r.Name, r.Protocol)
		}
	}
	return nil
}

// protocolsOverlap returns true if protocol a and b share any common transport.
func protocolsOverlap(a, b models.Protocol) bool {
	if a == models.ProtocolBoth || b == models.ProtocolBoth {
		return true
	}
	return a == b
}

func (m *Manager) startForwarders(r *models.ForwardRule) error {
	e := &entry{}
	if r.Protocol == models.ProtocolTCP || r.Protocol == models.ProtocolBoth {
		t := newTCPForwarder(r, m.cfg.DialTimeout, m.cfg.BufferSize)
		if err := t.Start(); err != nil {
			return err
		}
		e.tcp = t
	}
	if r.Protocol == models.ProtocolUDP || r.Protocol == models.ProtocolBoth {
		u := newUDPForwarder(r, m.cfg.UDPTimeout)
		if err := u.Start(); err != nil {
			if e.tcp != nil {
				e.tcp.Stop()
			}
			return err
		}
		e.udp = u
	}
	m.mu.Lock()
	m.active[r.ID] = e
	m.mu.Unlock()
	return nil
}

func (m *Manager) stopForwarders(id string) {
	m.mu.Lock()
	m.stopForwardersLocked(id)
	m.mu.Unlock()
}

func (m *Manager) stopForwardersLocked(id string) {
	e, ok := m.active[id]
	if !ok {
		return
	}
	if e.tcp != nil {
		e.tcp.Stop()
	}
	if e.udp != nil {
		e.udp.Stop()
	}
	delete(m.active, id)
}

func mergeStats(e *entry) (bytesIn, bytesOut, active, total int64) {
	if e.tcp != nil {
		bi, bo, a, t := e.tcp.Stats()
		bytesIn += bi
		bytesOut += bo
		active += a
		total += t
	}
	if e.udp != nil {
		bi, bo, a, t := e.udp.Stats()
		bytesIn += bi
		bytesOut += bo
		active += a
		total += t
	}
	return
}
