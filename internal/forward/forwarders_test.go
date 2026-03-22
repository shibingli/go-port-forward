package forward

import (
	"errors"
	"io"
	"net"
	"testing"
	"time"

	"go-port-forward/internal/logger"
	"go-port-forward/internal/models"
	"go.uber.org/zap"
)

func TestTCPForwarderStopClosesActiveConnections(t *testing.T) {
	logger.L = zap.NewNop()
	logger.S = logger.L.Sugar()

	targetLn, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen target: %v", err)
	}
	defer targetLn.Close()

	accepted := make(chan net.Conn, 1)
	go func() {
		conn, err := targetLn.Accept()
		if err == nil {
			accepted <- conn
		}
	}()

	rule := &models.ForwardRule{
		Name:       "tcp-stop",
		ListenAddr: "127.0.0.1",
		ListenPort: 0,
		TargetAddr: "127.0.0.1",
		TargetPort: targetLn.Addr().(*net.TCPAddr).Port,
	}
	fwd := newTCPForwarder(rule, 1, 4096)
	if err := fwd.Start(); err != nil {
		t.Fatalf("start tcp forwarder: %v", err)
	}

	client, err := net.Dial("tcp", fwd.listener.Addr().String())
	if err != nil {
		t.Fatalf("dial forwarder: %v", err)
	}
	defer client.Close()

	var upstream net.Conn
	select {
	case upstream = <-accepted:
		defer upstream.Close()
	case <-time.After(2 * time.Second):
		fwd.Stop()
		t.Fatal("timeout waiting for target accept")
	}

	stopped := make(chan struct{})
	go func() {
		fwd.Stop()
		close(stopped)
	}()

	select {
	case <-stopped:
	case <-time.After(2 * time.Second):
		t.Fatal("Stop did not return promptly")
	}

	_, _, active, total := fwd.Stats()
	if active != 0 {
		t.Fatalf("active connections after stop = %d, want 0", active)
	}
	if total != 1 {
		t.Fatalf("total connections = %d, want 1", total)
	}

	_ = client.SetReadDeadline(time.Now().Add(300 * time.Millisecond))
	buf := make([]byte, 1)
	_, err = client.Read(buf)
	if err == nil {
		t.Fatal("expected client read to fail after stop")
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		t.Fatalf("client connection was not closed after stop: %v", err)
	}
	if !errors.Is(err, io.EOF) {
		_ = client.SetWriteDeadline(time.Now().Add(300 * time.Millisecond))
		if _, werr := client.Write([]byte("x")); werr == nil {
			t.Fatalf("expected client write to fail after stop, read err=%v", err)
		}
	}
}

func TestUDPForwarderCleanupExpiresSessions(t *testing.T) {
	logger.L = zap.NewNop()
	logger.S = logger.L.Sugar()

	targetConn, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 0})
	if err != nil {
		t.Fatalf("listen udp target: %v", err)
	}
	defer targetConn.Close()

	stopEcho := make(chan struct{})
	go udpEchoLoop(targetConn, stopEcho)
	defer close(stopEcho)

	rule := &models.ForwardRule{
		Name:       "udp-cleanup",
		ListenAddr: "127.0.0.1",
		ListenPort: 0,
		TargetAddr: "127.0.0.1",
		TargetPort: targetConn.LocalAddr().(*net.UDPAddr).Port,
	}
	fwd := newUDPForwarder(rule, 1)
	if err := fwd.Start(); err != nil {
		t.Fatalf("start udp forwarder: %v", err)
	}
	defer fwd.Stop()

	client, err := net.DialUDP("udp", nil, fwd.conn.LocalAddr().(*net.UDPAddr))
	if err != nil {
		t.Fatalf("dial udp forwarder: %v", err)
	}
	defer client.Close()

	if _, err := client.Write([]byte("ping")); err != nil {
		t.Fatalf("write udp packet: %v", err)
	}
	_ = client.SetReadDeadline(time.Now().Add(2 * time.Second))
	buf := make([]byte, 16)
	n, err := client.Read(buf)
	if err != nil {
		t.Fatalf("read udp echo: %v", err)
	}
	if string(buf[:n]) != "ping" {
		t.Fatalf("unexpected udp echo %q", string(buf[:n]))
	}

	waitFor(t, 2*time.Second, func() bool {
		_, _, active, total := fwd.Stats()
		return active == 1 && total == 1
	})

	waitFor(t, 3*time.Second, func() bool {
		_, _, active, _ := fwd.Stats()
		return active == 0
	})
}

func udpEchoLoop(conn *net.UDPConn, stop <-chan struct{}) {
	buf := make([]byte, 2048)
	for {
		_ = conn.SetReadDeadline(time.Now().Add(200 * time.Millisecond))
		n, addr, err := conn.ReadFromUDP(buf)
		if err != nil {
			var netErr net.Error
			if errors.As(err, &netErr) && netErr.Timeout() {
				select {
				case <-stop:
					return
				default:
					continue
				}
			}
			return
		}
		_, _ = conn.WriteToUDP(buf[:n], addr)
	}
}

func waitFor(t *testing.T, timeout time.Duration, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatal("condition not satisfied before timeout")
}
