package gateway

import (
	"bufio"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"io"
	"math/big"
	"net"
	"os"
	"testing"
	"time"

	"huggingflowtransformers/internal/brandlog"
	"huggingflowtransformers/internal/config"
)

func TestGatewayForwardsTLSClientToUpstream(t *testing.T) {
	upstream := startEcho(t)
	cert, key := writeCert(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	cfg := config.GatewayConfig{
		Listen:               "127.0.0.1:9443",
		TLSCert:              cert,
		TLSKey:               key,
		CoordinationUpstream: upstream.Addr().String(),
		MaxSessions:          10,
		MaxSessionsPerIP:     10,
		ConnectTimeout:       time.Second,
		IdleTimeout:          time.Second,
		LogMode:              config.LogOff,
	}
	server := New(cfg, brandlog.New(brandlog.Options{Mode: brandlog.ModeOff}))
	go func() { _ = server.Serve(ctx) }()
	waitForPort(t, cfg.Listen)

	conn, err := tls.Dial("tcp", cfg.Listen, &tls.Config{InsecureSkipVerify: true, MinVersion: tls.VersionTLS13})
	if err != nil {
		t.Fatalf("tls dial: %v", err)
	}
	defer conn.Close()
	if _, err := conn.Write([]byte("hello\n")); err != nil {
		t.Fatalf("write: %v", err)
	}
	line, err := bufio.NewReader(conn).ReadString('\n')
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if line != "hello\n" {
		t.Fatalf("line = %q", line)
	}
}

func TestGatewayDoesNotConnectUpstreamBeforeTLSHandshake(t *testing.T) {
	upstream, accepted := startTrackingUpstream(t)
	cert, key := writeCert(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	cfg := config.GatewayConfig{
		Listen:               "127.0.0.1:9444",
		TLSCert:              cert,
		TLSKey:               key,
		CoordinationUpstream: upstream.Addr().String(),
		MaxSessions:          10,
		MaxSessionsPerIP:     10,
		ConnectTimeout:       50 * time.Millisecond,
		IdleTimeout:          time.Second,
		LogMode:              config.LogOff,
	}
	server := New(cfg, brandlog.New(brandlog.Options{Mode: brandlog.ModeOff}))
	go func() { _ = server.Serve(ctx) }()
	waitForPort(t, cfg.Listen)

	conn, err := net.Dial("tcp", cfg.Listen)
	if err != nil {
		t.Fatalf("dial raw gateway: %v", err)
	}
	defer conn.Close()

	select {
	case <-accepted:
		t.Fatal("gateway connected upstream before TLS handshake completed")
	case <-time.After(150 * time.Millisecond):
	}
}

func startEcho(t *testing.T) net.Listener {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen upstream: %v", err)
	}
	t.Cleanup(func() { _ = listener.Close() })
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			go func() {
				defer conn.Close()
				_, _ = io.Copy(conn, conn)
			}()
		}
	}()
	return listener
}

func startTrackingUpstream(t *testing.T) (net.Listener, <-chan struct{}) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen upstream: %v", err)
	}
	accepted := make(chan struct{}, 1)
	t.Cleanup(func() { _ = listener.Close() })
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			accepted <- struct{}{}
			_ = conn.Close()
		}
	}()
	return listener, accepted
}

func writeCert(t *testing.T) (string, string) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "localhost"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		DNSNames:     []string{"localhost"},
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1")},
		KeyUsage:     x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	der, err := x509.CreateCertificate(rand.Reader, &template, &template, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("create cert: %v", err)
	}
	dir := t.TempDir()
	certPath := dir + "/server.crt"
	keyPath := dir + "/server.key"
	if err := os.WriteFile(certPath, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0o600); err != nil {
		t.Fatalf("write cert: %v", err)
	}
	keyBytes := x509.MarshalPKCS1PrivateKey(key)
	if err := os.WriteFile(keyPath, pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: keyBytes}), 0o600); err != nil {
		t.Fatalf("write key: %v", err)
	}
	return certPath, keyPath
}

func waitForPort(t *testing.T, addr string) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", addr, 20*time.Millisecond)
		if err == nil {
			_ = conn.Close()
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("port %s did not open", addr)
}
