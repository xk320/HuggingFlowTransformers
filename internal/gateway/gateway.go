package gateway

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"
	"time"

	"huggingflowtransformers/internal/brandlog"
	"huggingflowtransformers/internal/config"
)

type Server struct {
	config config.GatewayConfig
	logger *brandlog.Logger
	mu     sync.Mutex
	total  int
	byIP   map[string]int
}

func New(cfg config.GatewayConfig, logger *brandlog.Logger) *Server {
	if logger == nil {
		logger = brandlog.New(brandlog.Options{Mode: brandlog.Mode(cfg.LogMode)})
	}
	return &Server{config: cfg, logger: logger, byIP: map[string]int{}}
}

func (s *Server) Serve(ctx context.Context) error {
	cert, err := tls.LoadX509KeyPair(s.config.TLSCert, s.config.TLSKey)
	if err != nil {
		return fmt.Errorf("load HFT gateway TLS certificate: %w", err)
	}
	listener, err := tls.Listen("tcp", s.config.Listen, &tls.Config{
		MinVersion:   tls.VersionTLS13,
		Certificates: []tls.Certificate{cert},
	})
	if err != nil {
		return fmt.Errorf("start HFT Secure Gateway: %w", err)
	}
	defer listener.Close()
	s.event("gateway_started", "info", map[string]string{"listen": listener.Addr().String()})

	go func() {
		<-ctx.Done()
		_ = listener.Close()
	}()

	for {
		conn, err := listener.Accept()
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return err
		}
		go s.acceptSession(ctx, conn)
	}
}

func (s *Server) acceptSession(ctx context.Context, conn net.Conn) {
	tlsConn, ok := conn.(*tls.Conn)
	if !ok {
		_ = conn.Close()
		return
	}
	if err := s.handshake(ctx, tlsConn); err != nil {
		s.event("session_handshake_failed", "warn", map[string]string{"client": shortClient(remoteIP(conn.RemoteAddr()))})
		_ = conn.Close()
		return
	}
	ip := remoteIP(conn.RemoteAddr())
	if !s.tryAcquire(ip) {
		s.event("session_limit_reached", "warn", map[string]string{"client": shortClient(ip)})
		_ = conn.Close()
		return
	}
	s.handle(ctx, ip, conn)
}

func (s *Server) handshake(ctx context.Context, conn *tls.Conn) error {
	timeout := s.config.ConnectTimeout
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	handshakeCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	_ = conn.SetDeadline(time.Now().Add(timeout))
	err := conn.HandshakeContext(handshakeCtx)
	_ = conn.SetDeadline(time.Time{})
	return err
}

func (s *Server) handle(ctx context.Context, ip string, client net.Conn) {
	defer s.release(ip)
	defer client.Close()
	s.event("session_accepted", "info", map[string]string{"client": shortClient(ip)})

	timeout := s.config.ConnectTimeout
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	dialer := net.Dialer{Timeout: timeout}
	upstream, err := dialer.DialContext(ctx, "tcp", s.config.CoordinationUpstream)
	if err != nil {
		s.event("upstream_unavailable", "warn", map[string]string{"error": err.Error()})
		return
	}
	defer upstream.Close()
	s.event("upstream_connected", "info", map[string]string{"client": shortClient(ip)})

	if s.config.IdleTimeout > 0 {
		client = deadlineConn{Conn: client, idle: s.config.IdleTimeout}
		upstream = deadlineConn{Conn: upstream, idle: s.config.IdleTimeout}
	}
	copyBoth(client, upstream)
	s.event("session_closed", "info", map[string]string{"client": shortClient(ip)})
}

func (s *Server) tryAcquire(ip string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.config.MaxSessions > 0 && s.total >= s.config.MaxSessions {
		return false
	}
	if s.config.MaxSessionsPerIP > 0 && s.byIP[ip] >= s.config.MaxSessionsPerIP {
		return false
	}
	s.total++
	s.byIP[ip]++
	return true
}

func (s *Server) release(ip string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.total > 0 {
		s.total--
	}
	if s.byIP[ip] > 1 {
		s.byIP[ip]--
	} else {
		delete(s.byIP, ip)
	}
}

func (s *Server) event(name, level string, fields map[string]string) {
	s.logger.Event(brandlog.Event{Time: time.Now().UTC(), Level: level, Device: -1, Name: name, Fields: fields})
}

func copyBoth(a, b net.Conn) {
	done := make(chan struct{}, 2)
	go func() {
		_, _ = io.Copy(a, b)
		done <- struct{}{}
	}()
	go func() {
		_, _ = io.Copy(b, a)
		done <- struct{}{}
	}()
	<-done
	_ = a.Close()
	_ = b.Close()
	<-done
}

func remoteIP(addr net.Addr) string {
	host, _, err := net.SplitHostPort(addr.String())
	if err != nil {
		return addr.String()
	}
	return host
}

func shortClient(ip string) string {
	ip = strings.TrimSpace(ip)
	if ip == "" {
		return "unknown"
	}
	if len(ip) <= 7 {
		return ip
	}
	return ip[:3] + "..." + ip[len(ip)-3:]
}

type deadlineConn struct {
	net.Conn
	idle time.Duration
}

func (c deadlineConn) Read(p []byte) (int, error) {
	_ = c.Conn.SetReadDeadline(time.Now().Add(c.idle))
	return c.Conn.Read(p)
}

func (c deadlineConn) Write(p []byte) (int, error) {
	_ = c.Conn.SetWriteDeadline(time.Now().Add(c.idle))
	return c.Conn.Write(p)
}
