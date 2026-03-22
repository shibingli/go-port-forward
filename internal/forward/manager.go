package forward

import (
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"

	"go-port-forward/internal/config"
	"go-port-forward/internal/logger"
	"go-port-forward/internal/models"
	"go-port-forward/internal/storage"
	"go-port-forward/pkg/pool"

	"github.com/google/uuid"
)

var (
	// ErrInvalidRule indicates invalid or out-of-range rule input.
	ErrInvalidRule = errors.New("invalid rule")
	// ErrPortConflict indicates listen address/port/protocol overlap.
	ErrPortConflict = errors.New("port conflict")
)

type entry struct {
	tcp *TCPForwarder
	udp *UDPForwarder
}

// Manager owns the lifecycle of all active forwarders.
type Manager struct {
	store           storage.Store
	rules           map[string]*models.ForwardRule
	active          map[string]*entry // rule ID → forwarders
	errors          map[string]string // rule ID → current error message
	lastErrors      map[string]string // rule ID → most recent error message
	errorTimes      map[string]time.Time
	errorCounts     map[string]int64
	statuses        map[string]models.RuleStatus
	statusChangedAt map[string]time.Time
	cfg             config.ForwardConfig
	opsMu           sync.Mutex
	mu              sync.RWMutex
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
		store:           store,
		cfg:             cfg,
		rules:           make(map[string]*models.ForwardRule),
		active:          make(map[string]*entry),
		errors:          make(map[string]string),
		lastErrors:      make(map[string]string),
		errorTimes:      make(map[string]time.Time),
		errorCounts:     make(map[string]int64),
		statuses:        make(map[string]models.RuleStatus),
		statusChangedAt: make(map[string]time.Time),
	}

	// Start all enabled rules persisted from a previous run.
	rules, err := store.ListRules()
	if err != nil {
		return nil, err
	}
	for _, r := range rules {
		m.rules[r.ID] = cloneRule(r)
		if r.Enabled {
			if e2 := m.startForwarders(r); e2 != nil {
				logger.S.Warnw("failed to start rule on boot", "rule", r.Name, "err", e2)
				m.setRuleError(r.ID, e2.Error())
			} else {
				m.recordRuleStatus(r.ID, models.StatusActive, time.Now())
			}
		} else {
			m.recordRuleStatus(r.ID, models.StatusInactive, statusAnchorTime(r))
		}
	}
	return m, nil
}

// ValidateCreateRequest normalizes and validates a create request without persisting it.
func (m *Manager) ValidateCreateRequest(req *models.CreateRuleRequest) error {
	if err := models.ValidateCreateRuleRequest(req); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidRule, err)
	}
	if err := m.checkPortConflict(req.ListenAddr, req.ListenPort, req.Protocol, ""); err != nil {
		return err
	}
	return nil
}

// AddRule validates, persists and starts a new rule.
func (m *Manager) AddRule(req *models.CreateRuleRequest) (*models.ForwardRule, error) {
	if req == nil {
		return nil, fmt.Errorf("%w: 请求不能为空 | request is required", ErrInvalidRule)
	}
	m.opsMu.Lock()
	defer m.opsMu.Unlock()

	normalized := *req
	if err := m.ValidateCreateRequest(&normalized); err != nil {
		return nil, err
	}
	r := &models.ForwardRule{
		ID:          uuid.NewString(),
		Name:        normalized.Name,
		ListenAddr:  normalized.ListenAddr,
		ListenPort:  normalized.ListenPort,
		Protocol:    normalized.Protocol,
		TargetAddr:  normalized.TargetAddr,
		TargetPort:  normalized.TargetPort,
		AddFirewall: normalized.AddFirewall,
		Comment:     normalized.Comment,
		Enabled:     normalized.Enabled,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	if err := m.store.SaveRule(r); err != nil {
		return nil, err
	}
	m.mu.Lock()
	m.rules[r.ID] = cloneRule(r)
	delete(m.errors, r.ID)
	m.mu.Unlock()
	if r.Enabled {
		if err := m.startForwarders(r); err != nil {
			m.setRuleError(r.ID, err.Error())
		} else {
			m.clearRuleError(r.ID)
			m.recordRuleStatus(r.ID, models.StatusActive, time.Now())
		}
	} else {
		m.recordRuleStatus(r.ID, models.StatusInactive, statusAnchorTime(r))
	}
	return m.decorateRule(r), nil
}

// UpdateRule applies partial updates to an existing rule (restarts forwarders as needed).
func (m *Manager) UpdateRule(id string, req *models.UpdateRuleRequest) (*models.ForwardRule, error) {
	if req == nil {
		return nil, fmt.Errorf("%w: 请求不能为空 | request is required", ErrInvalidRule)
	}
	m.opsMu.Lock()
	defer m.opsMu.Unlock()

	current, err := m.ruleFromCache(id)
	if err != nil {
		return nil, err
	}
	next := cloneRule(current)
	applyUpdate(next, req)
	next.UpdatedAt = time.Now()

	if err := models.ValidateForwardRule(next); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidRule, err)
	}

	// Port conflict detection (exclude self)
	if err := m.checkPortConflict(next.ListenAddr, next.ListenPort, next.Protocol, id); err != nil {
		return nil, err
	}

	if err := m.store.SaveRule(next); err != nil {
		return nil, err
	}
	m.mu.Lock()
	m.rules[id] = cloneRule(next)
	m.mu.Unlock()

	if !requiresForwarderRestart(current, next) {
		return m.decorateRule(next), nil
	}

	m.stopForwarders(id)
	statusChangedAt := time.Now()
	if next.Enabled {
		if err := m.startForwarders(next); err != nil {
			m.setRuleError(next.ID, err.Error())
		} else {
			m.clearRuleError(next.ID)
			m.recordRuleStatus(next.ID, models.StatusActive, statusChangedAt)
		}
	} else {
		m.clearRuleError(next.ID)
		m.recordRuleStatus(next.ID, models.StatusInactive, statusChangedAt)
	}
	return m.decorateRule(next), nil
}

// DeleteRule stops and removes a rule permanently.
func (m *Manager) DeleteRule(id string) error {
	m.opsMu.Lock()
	defer m.opsMu.Unlock()

	if _, err := m.ruleFromCache(id); err != nil {
		return err
	}
	if err := m.store.DeleteRule(id); err != nil {
		return err
	}
	m.stopForwarders(id)
	m.mu.Lock()
	delete(m.rules, id)
	delete(m.errors, id)
	delete(m.lastErrors, id)
	delete(m.errorTimes, id)
	delete(m.errorCounts, id)
	delete(m.statuses, id)
	delete(m.statusChangedAt, id)
	m.mu.Unlock()
	return nil
}

// ToggleRule enables or disables a rule.
func (m *Manager) ToggleRule(id string, enabled bool) (*models.ForwardRule, error) {
	on := enabled
	return m.UpdateRule(id, &models.UpdateRuleRequest{Enabled: &on})
}

// ListRules returns all rules with live stats merged in.
func (m *Manager) ListRules() ([]*models.ForwardRule, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	rules := m.ruleClonesLocked()
	for _, r := range rules {
		m.applyRuntimeStateLocked(r)
	}
	return rules, nil
}

// Snapshot returns current rules together with aggregated stats derived from the same snapshot.
func (m *Manager) Snapshot() ([]*models.ForwardRule, *models.Stats, error) {
	rules, err := m.ListRules()
	if err != nil {
		return nil, nil, err
	}
	return rules, buildStats(rules), nil
}

// GetRule returns one rule with live stats.
func (m *Manager) GetRule(id string) (*models.ForwardRule, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	r, ok := m.rules[id]
	if !ok {
		return nil, fmt.Errorf("%w: %s", storage.ErrRuleNotFound, id)
	}
	r = cloneRule(r)
	m.applyRuntimeStateLocked(r)
	return r, nil
}

// GlobalStats aggregates stats across all rules.
func (m *Manager) GlobalStats() *models.Stats {
	_, stats, err := m.Snapshot()
	if err != nil {
		return &models.Stats{}
	}
	return stats
}

// Diagnostics returns a lightweight operational snapshot for troubleshooting.
func (m *Manager) Diagnostics() (*models.ManagerDiagnostics, error) {
	rules, stats, err := m.Snapshot()
	if err != nil {
		return nil, err
	}
	d := &models.ManagerDiagnostics{
		CachedRules:     len(rules),
		Stats:           stats,
		HotRules:        make([]models.RuleTrafficSummary, 0),
		TopActiveRules:  make([]models.RuleTrafficSummary, 0),
		TopTrafficRules: make([]models.RuleTrafficSummary, 0),
		TopErrorRules:   make([]models.RuleErrorSummary, 0),
	}
	m.mu.RLock()
	d.ActiveForwarders = len(m.active)
	d.ErrorRules = len(m.errors)
	lastErrors := cloneStringMap(m.lastErrors)
	errorTimes := cloneTimeMap(m.errorTimes)
	errorCounts := cloneInt64Map(m.errorCounts)
	statusChangedAt := cloneTimeMap(m.statusChangedAt)
	for _, e := range m.active {
		if e == nil {
			continue
		}
		if e.tcp != nil {
			bi, bo, a, t := e.tcp.Stats()
			accumulateProtocolTraffic(&d.Protocols.TCP, bi, bo, a, t)
		}
		if e.udp != nil {
			bi, bo, a, t := e.udp.Stats()
			accumulateProtocolTraffic(&d.Protocols.UDP, bi, bo, a, t)
		}
	}
	m.mu.RUnlock()
	hotRules := make([]models.RuleTrafficSummary, 0, len(rules))
	activeRules := make([]models.RuleTrafficSummary, 0, len(rules))
	trafficRules := make([]models.RuleTrafficSummary, 0, len(rules))
	errorRules := make([]models.RuleErrorSummary, 0, len(rules))
	for _, r := range rules {
		addProtocolConfiguredCounts(&d.Protocols, r.Protocol)
		trafficSummary := buildRuleTrafficSummary(r, statusChangedAt[r.ID], errorTimes[r.ID])
		totalBytes := trafficSummary.TotalBytes
		if r.ActiveConns > 0 || r.TotalConns > 0 || totalBytes > 0 || r.Status == models.StatusActive {
			hotRules = append(hotRules, trafficSummary)
			activeRules = append(activeRules, trafficSummary)
			trafficRules = append(trafficRules, trafficSummary)
		}
		switch r.Status {
		case models.StatusActive:
			d.RuleHealth.Active++
		case models.StatusInactive:
			d.RuleHealth.Inactive++
		case models.StatusError:
			d.RuleHealth.Error++
			if r.ErrorMsg != "" {
				d.Errors = append(d.Errors, buildRuleErrorSummary(r, r.ErrorMsg, errorCounts[r.ID], statusChangedAt[r.ID], errorTimes[r.ID]))
			}
		}
		if errorCounts[r.ID] > 0 || !errorTimes[r.ID].IsZero() || lastErrors[r.ID] != "" {
			errorRules = append(errorRules, buildRuleErrorSummary(r, lastErrors[r.ID], errorCounts[r.ID], statusChangedAt[r.ID], errorTimes[r.ID]))
		}
	}
	sort.Slice(d.Errors, func(i, j int) bool {
		if !d.Errors[i].LastErrorAt.Equal(d.Errors[j].LastErrorAt) {
			return d.Errors[i].LastErrorAt.After(d.Errors[j].LastErrorAt)
		}
		if d.Errors[i].ErrorCount != d.Errors[j].ErrorCount {
			return d.Errors[i].ErrorCount > d.Errors[j].ErrorCount
		}
		if d.Errors[i].Name != d.Errors[j].Name {
			return d.Errors[i].Name < d.Errors[j].Name
		}
		if d.Errors[i].ListenPort != d.Errors[j].ListenPort {
			return d.Errors[i].ListenPort < d.Errors[j].ListenPort
		}
		return d.Errors[i].ID < d.Errors[j].ID
	})
	sort.Slice(hotRules, func(i, j int) bool {
		if hotRules[i].ActiveConns != hotRules[j].ActiveConns {
			return hotRules[i].ActiveConns > hotRules[j].ActiveConns
		}
		if hotRules[i].TotalBytes != hotRules[j].TotalBytes {
			return hotRules[i].TotalBytes > hotRules[j].TotalBytes
		}
		if hotRules[i].TotalConns != hotRules[j].TotalConns {
			return hotRules[i].TotalConns > hotRules[j].TotalConns
		}
		if hotRules[i].Name != hotRules[j].Name {
			return hotRules[i].Name < hotRules[j].Name
		}
		return hotRules[i].ID < hotRules[j].ID
	})
	sort.Slice(activeRules, func(i, j int) bool {
		if activeRules[i].ActiveConns != activeRules[j].ActiveConns {
			return activeRules[i].ActiveConns > activeRules[j].ActiveConns
		}
		if activeRules[i].TotalConns != activeRules[j].TotalConns {
			return activeRules[i].TotalConns > activeRules[j].TotalConns
		}
		if activeRules[i].TotalBytes != activeRules[j].TotalBytes {
			return activeRules[i].TotalBytes > activeRules[j].TotalBytes
		}
		if activeRules[i].Name != activeRules[j].Name {
			return activeRules[i].Name < activeRules[j].Name
		}
		return activeRules[i].ID < activeRules[j].ID
	})
	sort.Slice(trafficRules, func(i, j int) bool {
		if trafficRules[i].TotalBytes != trafficRules[j].TotalBytes {
			return trafficRules[i].TotalBytes > trafficRules[j].TotalBytes
		}
		if trafficRules[i].ActiveConns != trafficRules[j].ActiveConns {
			return trafficRules[i].ActiveConns > trafficRules[j].ActiveConns
		}
		if trafficRules[i].TotalConns != trafficRules[j].TotalConns {
			return trafficRules[i].TotalConns > trafficRules[j].TotalConns
		}
		if trafficRules[i].Name != trafficRules[j].Name {
			return trafficRules[i].Name < trafficRules[j].Name
		}
		return trafficRules[i].ID < trafficRules[j].ID
	})
	sort.Slice(errorRules, func(i, j int) bool {
		if errorRules[i].ErrorCount != errorRules[j].ErrorCount {
			return errorRules[i].ErrorCount > errorRules[j].ErrorCount
		}
		if !errorRules[i].LastErrorAt.Equal(errorRules[j].LastErrorAt) {
			return errorRules[i].LastErrorAt.After(errorRules[j].LastErrorAt)
		}
		if errorRules[i].Name != errorRules[j].Name {
			return errorRules[i].Name < errorRules[j].Name
		}
		return errorRules[i].ID < errorRules[j].ID
	})
	if len(hotRules) > 5 {
		hotRules = hotRules[:5]
	}
	if len(activeRules) > 5 {
		activeRules = activeRules[:5]
	}
	if len(trafficRules) > 5 {
		trafficRules = trafficRules[:5]
	}
	if len(errorRules) > 5 {
		errorRules = errorRules[:5]
	}
	d.HotRules = hotRules
	d.TopActiveRules = activeRules
	d.TopTrafficRules = trafficRules
	d.TopErrorRules = errorRules
	return d, nil
}

func buildRuleTrafficSummary(r *models.ForwardRule, lastStatusChangeAt, lastErrorAt time.Time) models.RuleTrafficSummary {
	return models.RuleTrafficSummary{
		ID:                 r.ID,
		Name:               r.Name,
		Protocol:           r.Protocol,
		Status:             r.Status,
		ListenAddr:         r.ListenAddr,
		ListenPort:         r.ListenPort,
		BytesIn:            r.BytesIn,
		BytesOut:           r.BytesOut,
		TotalBytes:         r.BytesIn + r.BytesOut,
		ActiveConns:        r.ActiveConns,
		TotalConns:         r.TotalConns,
		UpdatedAt:          r.UpdatedAt,
		LastErrorAt:        lastErrorAt,
		LastStatusChangeAt: lastStatusChangeAt,
	}
}

func buildRuleErrorSummary(r *models.ForwardRule, errMsg string, errCount int64, lastStatusChangeAt, lastErrorAt time.Time) models.RuleErrorSummary {
	return models.RuleErrorSummary{
		ID:                 r.ID,
		Name:               r.Name,
		Protocol:           r.Protocol,
		Status:             r.Status,
		ListenAddr:         r.ListenAddr,
		ListenPort:         r.ListenPort,
		Error:              errMsg,
		ErrorCount:         errCount,
		UpdatedAt:          r.UpdatedAt,
		LastErrorAt:        lastErrorAt,
		LastStatusChangeAt: lastStatusChangeAt,
	}
}

func cloneStringMap(src map[string]string) map[string]string {
	if len(src) == 0 {
		return map[string]string{}
	}
	dst := make(map[string]string, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

func cloneTimeMap(src map[string]time.Time) map[string]time.Time {
	if len(src) == 0 {
		return map[string]time.Time{}
	}
	dst := make(map[string]time.Time, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

func cloneInt64Map(src map[string]int64) map[string]int64 {
	if len(src) == 0 {
		return map[string]int64{}
	}
	dst := make(map[string]int64, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

func addProtocolConfiguredCounts(d *models.ProtocolBreakdownDiagnostics, proto models.Protocol) {
	switch models.NormalizeProtocol(proto) {
	case models.ProtocolTCP:
		d.TCP.ConfiguredRules++
	case models.ProtocolUDP:
		d.UDP.ConfiguredRules++
	case models.ProtocolBoth:
		d.TCP.ConfiguredRules++
		d.UDP.ConfiguredRules++
	}
}

func accumulateProtocolTraffic(d *models.ProtocolTrafficDiagnostics, bytesIn, bytesOut, active, total int64) {
	d.ActiveForwarders++
	d.BytesIn += bytesIn
	d.BytesOut += bytesOut
	d.ActiveConns += active
	d.TotalConns += total
}

func buildStats(rules []*models.ForwardRule) *models.Stats {
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
	addrA := models.NormalizeListenAddr(listenAddr)
	proto = models.NormalizeProtocol(proto)
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, r := range m.rules {
		if r.ID == excludeID {
			continue
		}
		addrB := models.NormalizeListenAddr(r.ListenAddr)
		if addrA != addrB || r.ListenPort != listenPort {
			continue
		}
		if protocolsOverlap(proto, r.Protocol) {
			return fmt.Errorf("%w: %s:%d 已被规则 | already used by rule %q 占用 (协议 | protocol %s)",
				ErrPortConflict,
				addrA, listenPort, r.Name, r.Protocol)
		}
	}
	return nil
}

// protocolsOverlap returns true if protocol a and b share any common transport.
func protocolsOverlap(a, b models.Protocol) bool {
	a = models.NormalizeProtocol(a)
	b = models.NormalizeProtocol(b)
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

func cloneRule(r *models.ForwardRule) *models.ForwardRule {
	if r == nil {
		return nil
	}
	clone := *r
	return &clone
}

func (m *Manager) ruleFromCache(id string) (*models.ForwardRule, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	r, ok := m.rules[id]
	if !ok {
		return nil, fmt.Errorf("%w: %s", storage.ErrRuleNotFound, id)
	}
	return cloneRule(r), nil
}

func (m *Manager) ruleClonesLocked() []*models.ForwardRule {
	rules := make([]*models.ForwardRule, 0, len(m.rules))
	for _, r := range m.rules {
		if r != nil {
			rules = append(rules, cloneRule(r))
		}
	}
	sort.Slice(rules, func(i, j int) bool {
		if !rules[i].CreatedAt.Equal(rules[j].CreatedAt) {
			return rules[i].CreatedAt.Before(rules[j].CreatedAt)
		}
		if rules[i].Name != rules[j].Name {
			return rules[i].Name < rules[j].Name
		}
		return rules[i].ID < rules[j].ID
	})
	return rules
}

func (m *Manager) applyRuntimeStateLocked(r *models.ForwardRule) {
	if e, ok := m.active[r.ID]; ok {
		r.Status = models.StatusActive
		r.ErrorMsg = ""
		r.BytesIn, r.BytesOut, r.ActiveConns, r.TotalConns = mergeStats(e)
		return
	}
	if r.Enabled {
		r.Status = models.StatusError
		r.ErrorMsg = m.errors[r.ID]
		return
	}
	r.Status = models.StatusInactive
	r.ErrorMsg = ""
}

func (m *Manager) decorateRule(r *models.ForwardRule) *models.ForwardRule {
	r = cloneRule(r)
	m.mu.RLock()
	m.applyRuntimeStateLocked(r)
	m.mu.RUnlock()
	return r
}

func (m *Manager) setRuleError(id, msg string) {
	now := time.Now()
	m.mu.Lock()
	m.errors[id] = msg
	m.lastErrors[id] = msg
	m.errorTimes[id] = now
	m.errorCounts[id]++
	m.recordRuleStatusLocked(id, models.StatusError, now)
	m.mu.Unlock()
}

func (m *Manager) clearRuleError(id string) {
	m.mu.Lock()
	delete(m.errors, id)
	m.mu.Unlock()
}

func (m *Manager) recordRuleStatus(id string, status models.RuleStatus, at time.Time) {
	m.mu.Lock()
	m.recordRuleStatusLocked(id, status, at)
	m.mu.Unlock()
}

func (m *Manager) recordRuleStatusLocked(id string, status models.RuleStatus, at time.Time) {
	if at.IsZero() {
		at = time.Now()
	}
	prev, ok := m.statuses[id]
	if !ok || prev != status {
		m.statuses[id] = status
		m.statusChangedAt[id] = at
		return
	}
	if _, exists := m.statusChangedAt[id]; !exists {
		m.statusChangedAt[id] = at
	}
}

func statusAnchorTime(r *models.ForwardRule) time.Time {
	if r == nil {
		return time.Time{}
	}
	if !r.UpdatedAt.IsZero() {
		return r.UpdatedAt
	}
	return r.CreatedAt
}

func applyUpdate(r *models.ForwardRule, req *models.UpdateRuleRequest) {
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
}

func requiresForwarderRestart(before, after *models.ForwardRule) bool {
	if before == nil || after == nil {
		return true
	}
	return before.Enabled != after.Enabled ||
		models.NormalizeListenAddr(before.ListenAddr) != models.NormalizeListenAddr(after.ListenAddr) ||
		before.ListenPort != after.ListenPort ||
		models.NormalizeProtocol(before.Protocol) != models.NormalizeProtocol(after.Protocol) ||
		before.TargetAddr != after.TargetAddr ||
		before.TargetPort != after.TargetPort
}
