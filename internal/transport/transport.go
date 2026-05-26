package transport

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	_ "embed"
	"fmt"
	"net"
	"net/url"
	"os"
	"strings"
	"time"

	"huggingflowtransformers/internal/config"
)

//go:embed default_gateway_ca.pem
var defaultGatewayCAPEM []byte

const (
	defaultGatewayHost = "38.76.221.73"
	defaultGatewayPort = "8443"
)

type Dialer struct {
	Config config.Config
}

func (d Dialer) Dial(ctx context.Context) (net.Conn, error) {
	parsed, err := url.Parse(d.Config.GatewayURL)
	if err != nil || parsed.Scheme != "tls" || parsed.Host == "" {
		return nil, fmt.Errorf("HFT secure transport requires tls:// gateway URL")
	}
	timeout := d.Config.GatewayConnectTimeout
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	dialer := &net.Dialer{Timeout: timeout}
	raw, err := dialer.DialContext(ctx, "tcp", parsed.Host)
	if err != nil {
		return nil, fmt.Errorf("connect HFT Secure Gateway: %w", err)
	}
	tlsConfig, err := TLSConfig(d.Config, parsed)
	if err != nil {
		_ = raw.Close()
		return nil, err
	}
	conn := tls.Client(raw, tlsConfig)
	if deadline, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(deadline)
	} else {
		_ = conn.SetDeadline(time.Now().Add(timeout))
	}
	if err := conn.HandshakeContext(ctx); err != nil {
		_ = raw.Close()
		return nil, fmt.Errorf("establish HFT Secure Transport: %w", err)
	}
	_ = conn.SetDeadline(time.Time{})
	return conn, nil
}

func DialDirect(ctx context.Context, baseURL string, timeout time.Duration) (net.Conn, error) {
	parsed, err := url.Parse(baseURL)
	if err != nil || parsed.Scheme != "stratum+tcp" || parsed.Host == "" {
		return nil, fmt.Errorf("HFT direct transport requires stratum+tcp://host:port base URL")
	}
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	dialer := &net.Dialer{Timeout: timeout}
	conn, err := dialer.DialContext(ctx, "tcp", parsed.Host)
	if err != nil {
		return nil, fmt.Errorf("connect HFT Coordination endpoint: %w", err)
	}
	return conn, nil
}

func TLSConfig(cfg config.Config, gateway *url.URL) (*tls.Config, error) {
	serverName := strings.TrimSpace(cfg.GatewayServerName)
	if serverName == "" && gateway != nil {
		serverName = gateway.Hostname()
	}
	tlsConfig := &tls.Config{
		MinVersion: tls.VersionTLS13,
		ServerName: serverName,
	}
	if cfg.GatewayCAFile != "" {
		pool, err := certPoolFromPEMFile(cfg.GatewayCAFile)
		if err != nil {
			return nil, err
		}
		tlsConfig.RootCAs = pool
		return tlsConfig, nil
	}
	if isDefaultGateway(gateway) {
		pool, err := certPoolFromPEM(defaultGatewayCAPEM, "embedded HFT default gateway CA")
		if err != nil {
			return nil, err
		}
		tlsConfig.RootCAs = pool
	}
	return tlsConfig, nil
}

func isDefaultGateway(gateway *url.URL) bool {
	if gateway == nil {
		return false
	}
	return gateway.Hostname() == defaultGatewayHost && gateway.Port() == defaultGatewayPort
}

func certPoolFromPEMFile(path string) (*x509.CertPool, error) {
	pem, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read HFT gateway CA file: %w", err)
	}
	return certPoolFromPEM(pem, "HFT gateway CA file")
}

func certPoolFromPEM(pem []byte, label string) (*x509.CertPool, error) {
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(pem) {
		return nil, fmt.Errorf("%s contains no certificates", label)
	}
	return pool, nil
}
