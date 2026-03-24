package forward

import (
	"errors"
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"go-port-forward/internal/logger"
	"go-port-forward/internal/models"
	"go-port-forward/pkg/pool"

	"go.uber.org/zap"
)

// udpAddrKey 是 UDP 地址的固定大小可比较 key，避免每包调用 String() 产生字符串分配。
// udpAddrKey is a fixed-size comparable key for UDP addresses, avoiding per-packet
// string allocation from String().
type udpAddrKey struct {
	ip   [net.IPv6len]byte // 16 bytes，同时容纳 IPv4 和 IPv6 | fits both IPv4 and IPv6
	port uint16
	len  uint8 // IP 原始长度（4 或 16），确保不同表示不会碰撞 | raw IP length (4 or 16)
	zone string
}

// makeUDPAddrKey 从 *net.UDPAddr 构造零分配的 map key。
// makeUDPAddrKey constructs a zero-allocation map key from *net.UDPAddr.
func makeUDPAddrKey(addr *net.UDPAddr) udpAddrKey {
	var k udpAddrKey
	k.port = uint16(addr.Port)
	k.len = uint8(len(addr.IP))
	k.zone = addr.Zone
	copy(k.ip[:], addr.IP)
	return k
}

// udpSession tracks an upstream UDP connection for a specific client address.
type udpSession struct {
	upstream *net.UDPConn
	lastSeen time.Time
}

// UDPForwarder listens on a local UDP port and forwards datagrams to a target.
type UDPForwarder struct {
	rule       *models.ForwardRule
	conn       *net.UDPConn
	targetAddr *net.UDPAddr
	sessions   map[udpAddrKey]*udpSession
	stopCh     chan struct{}
	wg         sync.WaitGroup
	timeout    time.Duration
	bytesIn    atomic.Int64
	bytesOut   atomic.Int64
	active     atomic.Int64
	totalConns atomic.Int64
	stopOnce   sync.Once
	mu         sync.Mutex
}

func newUDPForwarder(rule *models.ForwardRule, timeoutSec int) *UDPForwarder {
	if timeoutSec <= 0 {
		timeoutSec = 30
	}
	return &UDPForwarder{
		rule:     rule,
		timeout:  time.Duration(timeoutSec) * time.Second,
		sessions: make(map[udpAddrKey]*udpSession),
		stopCh:   make(chan struct{}),
	}
}

func (f *UDPForwarder) Start() error {
	listenAddr := fmt.Sprintf("%s:%d", f.rule.ListenAddr, f.rule.ListenPort)
	addr, err := net.ResolveUDPAddr("udp", listenAddr)
	if err != nil {
		return err
	}
	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		return fmt.Errorf("UDP 监听失败 | UDP listen failed %s: %w", listenAddr, err)
	}
	targetAddr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%d", f.rule.TargetAddr, f.rule.TargetPort))
	if err != nil {
		_ = conn.Close()
		return fmt.Errorf("UDP 目标地址无效 | invalid UDP target address: %w", err)
	}
	f.conn = conn
	f.targetAddr = targetAddr

	f.wg.Add(2)
	go f.readLoop()
	go f.cleanupLoop()

	logger.S.Infow("UDP forwarder started", "rule", f.rule.Name, "listen", listenAddr,
		"target", fmt.Sprintf("%s:%d", f.rule.TargetAddr, f.rule.TargetPort))
	return nil
}

func (f *UDPForwarder) Stop() {
	f.stopOnce.Do(func() {
		close(f.stopCh)
		if f.conn != nil {
			_ = f.conn.Close()
		}
		f.mu.Lock()
		for key, s := range f.sessions {
			_ = s.upstream.Close()
			delete(f.sessions, key)
		}
		f.active.Store(0)
		f.mu.Unlock()
	})
	f.wg.Wait()
}

func (f *UDPForwarder) readLoop() {
	defer f.wg.Done()
	// Use pooled buffer for reading
	buf := pool.GetBuffer(65535)
	defer pool.PutBuffer(buf)
	for {
		n, srcAddr, err := f.conn.ReadFromUDP(buf)
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				return
			}
			select {
			case <-f.stopCh:
				return
			default:
				if ne, ok := errors.AsType[net.Error](err); ok && ne.Temporary() {
					time.Sleep(50 * time.Millisecond)
					continue
				}
				logger.L.Warn("UDP read error", zap.Error(err))
				return
			}
		}
		// Copy packet data for async processing
		pkt := pool.GetBuffer(n)[:n]
		copy(pkt, buf[:n])
		// Use goroutine pool via pkg/pool
		if err := pool.Submit(func() { f.forward(srcAddr, pkt) }); err != nil {
			go f.forward(srcAddr, pkt)
		}
	}
}

func (f *UDPForwarder) forward(srcAddr *net.UDPAddr, data []byte) {
	defer pool.PutBuffer(data)
	if f.isStopping() {
		return
	}
	sess := f.getOrCreateSession(srcAddr)
	if sess == nil {
		return
	}

	n, _ := sess.upstream.Write(data)
	f.bytesIn.Add(int64(n))
}

func (f *UDPForwarder) relayBack(clientAddr *net.UDPAddr, sess *udpSession) {
	defer f.wg.Done()
	// Use pooled buffer for relay
	buf := pool.GetBuffer(65535)
	defer pool.PutBuffer(buf)
	for {
		n, err := sess.upstream.Read(buf)
		if err != nil {
			return
		}
		out, _ := f.conn.WriteToUDP(buf[:n], clientAddr)
		f.bytesOut.Add(int64(out))
	}
}

func (f *UDPForwarder) cleanupLoop() {
	defer f.wg.Done()
	ticker := time.NewTicker(f.timeout / 2)
	defer ticker.Stop()
	for {
		select {
		case <-f.stopCh:
			return
		case <-ticker.C:
			f.mu.Lock()
			now := time.Now()
			for k, s := range f.sessions {
				if now.Sub(s.lastSeen) > f.timeout {
					_ = s.upstream.Close()
					delete(f.sessions, k)
					f.active.Add(-1)
				}
			}
			f.mu.Unlock()
		}
	}
}

func (f *UDPForwarder) Stats() (bytesIn, bytesOut, active, total int64) {
	return f.bytesIn.Load(), f.bytesOut.Load(), f.active.Load(), f.totalConns.Load()
}

func (f *UDPForwarder) getOrCreateSession(srcAddr *net.UDPAddr) *udpSession {
	key := makeUDPAddrKey(srcAddr) // 零分配 | zero allocation
	now := time.Now()

	f.mu.Lock()
	if sess, ok := f.sessions[key]; ok {
		sess.lastSeen = now
		f.mu.Unlock()
		return sess
	}
	f.mu.Unlock()
	if f.isStopping() {
		return nil
	}

	up, err := net.DialUDP("udp", nil, f.targetAddr)
	if err != nil {
		logger.L.Warn("UDP dial failed", zap.String("target", f.targetAddr.String()), zap.Error(err))
		return nil
	}

	f.mu.Lock()
	if sess, ok := f.sessions[key]; ok {
		sess.lastSeen = now
		f.mu.Unlock()
		_ = up.Close()
		return sess
	}
	if f.isStopping() {
		f.mu.Unlock()
		_ = up.Close()
		return nil
	}
	sess := &udpSession{upstream: up, lastSeen: now}
	f.sessions[key] = sess
	f.active.Add(1)
	f.totalConns.Add(1)
	f.wg.Add(1)
	go f.relayBack(cloneUDPAddr(srcAddr), sess)
	f.mu.Unlock()
	return sess
}

func cloneUDPAddr(addr *net.UDPAddr) *net.UDPAddr {
	if addr == nil {
		return nil
	}
	clone := *addr
	if addr.IP != nil {
		clone.IP = append(net.IP(nil), addr.IP...)
	}
	return &clone
}

func (f *UDPForwarder) isStopping() bool {
	select {
	case <-f.stopCh:
		return true
	default:
		return false
	}
}
