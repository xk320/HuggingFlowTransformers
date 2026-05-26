package transport

import (
	"context"
	"encoding/pem"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"testing"
	"time"

	"huggingflowtransformers/internal/config"
)

func TestTLSConfigUsesGatewayHost(t *testing.T) {
	parsed, _ := url.Parse("tls://gateway.example.com:8443")
	cfg, err := TLSConfig(config.Config{}, parsed)
	if err != nil {
		t.Fatalf("TLSConfig returned error: %v", err)
	}
	if cfg.ServerName != "gateway.example.com" {
		t.Fatalf("ServerName = %q", cfg.ServerName)
	}
	if cfg.RootCAs != nil {
		t.Fatal("expected custom gateway to use system roots by default")
	}
}

func TestTLSConfigUsesEmbeddedCAForDefaultGateway(t *testing.T) {
	parsed, _ := url.Parse("tls://38.76.221.73:8443")
	cfg, err := TLSConfig(config.Config{}, parsed)
	if err != nil {
		t.Fatalf("TLSConfig returned error: %v", err)
	}
	if cfg.RootCAs == nil {
		t.Fatal("expected embedded default gateway CA pool")
	}
}

func TestDialRejectsNonTLSURL(t *testing.T) {
	_, err := Dialer{Config: config.Config{GatewayURL: "tcp://127.0.0.1:1234"}}.Dial(context.Background())
	if err == nil {
		t.Fatal("expected non TLS URL to fail")
	}
}

func TestDialDirectRejectsNonStratumURL(t *testing.T) {
	_, err := DialDirect(context.Background(), "tls://127.0.0.1:1234", time.Second)
	if err == nil {
		t.Fatal("expected non stratum URL to fail")
	}
}

func TestDialDirectConnectsToEndpoint(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer listener.Close()
	go func() {
		conn, err := listener.Accept()
		if err == nil {
			_ = conn.Close()
		}
	}()

	conn, err := DialDirect(context.Background(), "stratum+tcp://"+listener.Addr().String(), time.Second)
	if err != nil {
		t.Fatalf("DialDirect returned error: %v", err)
	}
	_ = conn.Close()
}

func TestDialConnectsToTLSServer(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("ok"))
	}))
	defer server.Close()
	parsed, _ := url.Parse(server.URL)
	caFile := t.TempDir() + "/ca.pem"
	if err := os.WriteFile(caFile, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: server.Certificate().Raw}), 0o600); err != nil {
		t.Fatalf("write ca file: %v", err)
	}

	cfg := config.Config{
		GatewayURL:            "tls://" + parsed.Host,
		GatewayServerName:     "example.com",
		GatewayCAFile:         caFile,
		GatewayConnectTimeout: time.Second,
	}
	conn, err := Dialer{Config: cfg}.Dial(context.Background())
	if err != nil {
		t.Fatalf("Dial returned error: %v", err)
	}
	defer conn.Close()
}

var _ net.Conn
