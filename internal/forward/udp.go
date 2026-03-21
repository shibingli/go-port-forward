package forward

import (
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

// udpSession tracks an upstream UDP connection for a specific client address.
type udpSession struct {
	upstream *net.UDPConn
	lastSeen time.Time
}

// UDPForwarder listens on a local UDP port and forwards datagrams to a target.
type UDPForwarder struct {
	rule       *models.ForwardRule
	conn       *net.UDPConn
	timeout    time.Duration
	sessions   map[string]*udpSession
	mu         sync.Mutex
	stopCh     chan struct{}
	stopOnce   sync.Once
	wg         sync.WaitGroup
	bytesIn    atomic.Int64
	bytesOut   atomic.Int64
	totalConns atomic.Int64
}

func newUDPForwarder(rule *models.ForwardRule, timeoutSec int) *UDPForwarder {
	return &UDPForwarder{
		rule:     rule,
		timeout:  time.Duration(timeoutSec) * time.Second,
		sessions: make(map[string]*udpSession),
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
	f.conn = conn

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
		for _, s := range f.sessions {
			_ = s.upstream.Close()
		}
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
			select {
			case <-f.stopCh:
				return
			default:
				logger.L.Warn("UDP read error", zap.Error(err))
				return
			}
		}
		// Copy packet data for async processing
		pkt := make([]byte, n)
		copy(pkt, buf[:n])
		// Use goroutine pool via pkg/pool
		if err := pool.Submit(func() { f.forward(srcAddr, pkt) }); err != nil {
			go f.forward(srcAddr, pkt)
		}
	}
}

func (f *UDPForwarder) forward(srcAddr *net.UDPAddr, data []byte) {
	key := srcAddr.String()
	f.mu.Lock()
	sess, ok := f.sessions[key]
	if !ok {
		targetAddr := fmt.Sprintf("%s:%d", f.rule.TargetAddr, f.rule.TargetPort)
		upAddr, err := net.ResolveUDPAddr("udp", targetAddr)
		if err != nil {
			f.mu.Unlock()
			return
		}
		up, err := net.DialUDP("udp", nil, upAddr)
		if err != nil {
			f.mu.Unlock()
			logger.L.Warn("UDP dial failed", zap.String("target", targetAddr), zap.Error(err))
			return
		}
		sess = &udpSession{upstream: up, lastSeen: time.Now()}
		f.sessions[key] = sess
		f.totalConns.Add(1)
		// relay replies back to client
		go f.relayBack(srcAddr, sess)
	}
	sess.lastSeen = time.Now()
	f.mu.Unlock()

	n, _ := sess.upstream.Write(data)
	f.bytesIn.Add(int64(n))
}

func (f *UDPForwarder) relayBack(clientAddr *net.UDPAddr, sess *udpSession) {
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
				}
			}
			f.mu.Unlock()
		}
	}
}

func (f *UDPForwarder) Stats() (bytesIn, bytesOut, active, total int64) {
	f.mu.Lock()
	active = int64(len(f.sessions))
	f.mu.Unlock()
	return f.bytesIn.Load(), f.bytesOut.Load(), active, f.totalConns.Load()
}
