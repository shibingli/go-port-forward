package forward

import (
	"context"
	"fmt"
	"io"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"go-port-forward/internal/logger"
	"go-port-forward/internal/models"
	"go-port-forward/pkg/pool"
	"go-port-forward/pkg/retry"

	"go.uber.org/zap"
)

// TCPForwarder listens on a local TCP port and forwards connections to a target.
type TCPForwarder struct {
	rule        *models.ForwardRule
	dialTimeout time.Duration
	bufferSize  int
	listener    net.Listener
	cancel      context.CancelFunc
	wg          sync.WaitGroup

	// stats (atomic)
	bytesIn     atomic.Int64
	bytesOut    atomic.Int64
	activeConns atomic.Int64
	totalConns  atomic.Int64
}

func newTCPForwarder(rule *models.ForwardRule, dialTimeoutSec, bufferSize int) *TCPForwarder {
	if dialTimeoutSec <= 0 {
		dialTimeoutSec = 10
	}
	if bufferSize <= 0 {
		bufferSize = pool.DefaultBufferSize
	}
	return &TCPForwarder{
		rule:        rule,
		dialTimeout: time.Duration(dialTimeoutSec) * time.Second,
		bufferSize:  bufferSize,
	}
}

func (f *TCPForwarder) Start() error {
	listenAddr := fmt.Sprintf("%s:%d", f.rule.ListenAddr, f.rule.ListenPort)
	ln, err := net.Listen("tcp", listenAddr)
	if err != nil {
		return fmt.Errorf("TCP 监听失败 | TCP listen failed %s: %w", listenAddr, err)
	}
	f.listener = ln

	ctx, cancel := context.WithCancel(context.Background())
	f.cancel = cancel

	f.wg.Add(1)
	go f.acceptLoop(ctx)
	logger.S.Infow("TCP forwarder started", "rule", f.rule.Name, "listen", listenAddr,
		"target", fmt.Sprintf("%s:%d", f.rule.TargetAddr, f.rule.TargetPort))
	return nil
}

func (f *TCPForwarder) Stop() {
	if f.cancel != nil {
		f.cancel()
	}
	if f.listener != nil {
		_ = f.listener.Close()
	}
	f.wg.Wait()
}

func (f *TCPForwarder) acceptLoop(ctx context.Context) {
	defer f.wg.Done()
	for {
		conn, err := f.listener.Accept()
		if err != nil {
			select {
			case <-ctx.Done():
				return
			default:
				logger.S.Warnw("TCP accept error", "rule", f.rule.Name, "err", err)
				return
			}
		}
		rule := f.rule
		c := conn
		// Use global goroutine pool via pkg/pool
		if err := pool.Submit(func() { f.handleConn(ctx, c, rule) }); err != nil {
			logger.L.Warn("pool submit failed, running in new goroutine", zap.Error(err))
			go f.handleConn(ctx, c, rule)
		}
	}
}

func (f *TCPForwarder) handleConn(ctx context.Context, src net.Conn, rule *models.ForwardRule) {
	defer src.Close()
	f.activeConns.Add(1)
	f.totalConns.Add(1)
	defer f.activeConns.Add(-1)

	target := fmt.Sprintf("%s:%d", rule.TargetAddr, rule.TargetPort)

	// Dial with retry (exponential backoff, max 3 retries, capped at 5s)
	var dst net.Conn
	err := retry.DoWithExponentialCapped(ctx, 3, 500*time.Millisecond, 5*time.Second,
		func(retryCtx context.Context) error {
			dialer := &net.Dialer{Timeout: f.dialTimeout}
			conn, dialErr := dialer.DialContext(retryCtx, "tcp", target)
			if dialErr != nil {
				return retry.RetryableError(dialErr)
			}
			dst = conn
			return nil
		})
	if err != nil {
		logger.L.Warn("TCP dial failed after retries", zap.String("target", target), zap.Error(err))
		return
	}
	defer dst.Close()

	var wg sync.WaitGroup
	wg.Add(2)

	// client → target: after EOF from client, half-close the target write side
	go func() {
		defer wg.Done()
		n := f.copyBuf(dst, src)
		f.bytesIn.Add(n)
		if tc, ok := dst.(*net.TCPConn); ok {
			_ = tc.CloseWrite()
		}
	}()
	// target → client: after EOF from target, half-close the client write side
	go func() {
		defer wg.Done()
		n := f.copyBuf(src, dst)
		f.bytesOut.Add(n)
		if tc, ok := src.(*net.TCPConn); ok {
			_ = tc.CloseWrite()
		}
	}()
	wg.Wait()
}

// copyBuf copies from src to dst using a pooled buffer from pkg/pool; returns bytes written.
func (f *TCPForwarder) copyBuf(dst io.Writer, src io.Reader) int64 {
	buf := pool.GetBuffer(f.bufferSize)
	defer pool.PutBuffer(buf)
	n, _ := io.CopyBuffer(dst, src, buf)
	return n
}

func (f *TCPForwarder) Stats() (bytesIn, bytesOut, active, total int64) {
	return f.bytesIn.Load(), f.bytesOut.Load(), f.activeConns.Load(), f.totalConns.Load()
}
