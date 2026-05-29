package config

import (
	"os"
	"reflect"
	"testing"
	"time"
)

func TestLoadDefaultsUseOnlyBrandedEnvironment(t *testing.T) {
	t.Setenv("HFT_BASE_URL", "stratum+tcp://example.invalid:5566")
	t.Setenv("HFT_KEY", "sk_testvalue")
	t.Setenv("HFT_DEVICES", "0,2")
	t.Setenv("HFT_NODE_PREFIX", "Node A")
	t.Setenv("HFT_LOG_MODE", "stdout")
	t.Setenv("ALPHA_POOL", "should-not-be-read")

	cfg, err := Load([]string{"HuggingFlowTransformers"})
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	if cfg.BaseURL != "stratum+tcp://example.invalid:5566" {
		t.Fatalf("BaseURL = %q", cfg.BaseURL)
	}
	if cfg.InternalKey != "prl1ptestvalue" {
		t.Fatalf("InternalKey = %q", cfg.InternalKey)
	}
	if cfg.DisplayKey == cfg.InternalKey {
		t.Fatalf("DisplayKey should not equal InternalKey")
	}
	if !reflect.DeepEqual(cfg.Devices, []int{0, 2}) {
		t.Fatalf("Devices = %#v", cfg.Devices)
	}
	if cfg.NodeName(2) != "Node-A-g2" {
		t.Fatalf("NodeName(2) = %q", cfg.NodeName(2))
	}
	if cfg.LogMode != LogStdout {
		t.Fatalf("LogMode = %q", cfg.LogMode)
	}
	if cfg.CompatMode {
		t.Fatal("CompatMode should default to false")
	}
	if cfg.ModelDataTimeout == 0 {
		t.Fatal("ModelDataTimeout should have a default")
	}
}

func TestLoadAllowsCompatModeOptIn(t *testing.T) {
	t.Setenv("HFT_COMPAT_MODE", "true")

	cfg, err := Load([]string{"HuggingFlowTransformers"})
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if !cfg.CompatMode {
		t.Fatal("CompatMode should be enabled when HFT_COMPAT_MODE=true")
	}
}

func TestLoadRejectsCommandLineArguments(t *testing.T) {
	_, err := Load([]string{"HuggingFlowTransformers", "--help"})
	if err == nil {
		t.Fatal("expected command line arguments to be rejected")
	}
}

func TestLoadRejectsUnknownKeyPrefix(t *testing.T) {
	t.Setenv("HFT_KEY", "prl1ptestvalue")

	_, err := Load([]string{"HuggingFlowTransformers"})
	if err == nil {
		t.Fatal("expected non sk_ key to be rejected")
	}
}

func TestLoadDevicesAll(t *testing.T) {
	t.Setenv("HFT_DEVICES", "all")

	cfg, err := Load([]string{"HuggingFlowTransformers"})
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.Devices != nil {
		t.Fatalf("all devices should be represented as nil, got %#v", cfg.Devices)
	}
}

func TestCleanHostnameFallback(t *testing.T) {
	t.Setenv("HFT_NODE_PREFIX", "  ###  ")

	cfg, err := Load([]string{"HuggingFlowTransformers"})
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	host, _ := os.Hostname()
	if got := cfg.NodeName(0); got == "" || got == host {
		t.Fatalf("NodeName should be cleaned and include device suffix, got %q", got)
	}
}

func TestLoadUsesBrandedModelDataTimeout(t *testing.T) {
	t.Setenv("HFT_MODEL_DATA_TIMEOUT", "123")
	t.Setenv("HFT_NO_MODEL_DATA_TIMEOUT", "456")
	t.Setenv("HFT_REPORT", "true")

	cfg, err := Load([]string{"HuggingFlowTransformers"})
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if got := cfg.ModelDataTimeout.Seconds(); got != 123 {
		t.Fatalf("ModelDataTimeout seconds = %v", got)
	}
	if !cfg.Report {
		t.Fatal("Report should be enabled")
	}
}

func TestLoadKeepsLegacyNoModelDataTimeoutCompatibility(t *testing.T) {
	t.Setenv("HFT_NO_MODEL_DATA_TIMEOUT", "456")

	cfg, err := Load([]string{"HuggingFlowTransformers"})
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if got := cfg.ModelDataTimeout.Seconds(); got != 456 {
		t.Fatalf("ModelDataTimeout seconds = %v", got)
	}
}

func TestLoadUsesDefaultGatewayURL(t *testing.T) {
	t.Setenv("HFT_GATEWAY_MODE", "on")

	cfg, err := Load([]string{"HuggingFlowTransformers"})
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.GatewayURL != defaultGatewayURL {
		t.Fatalf("GatewayURL = %q", cfg.GatewayURL)
	}
}

func TestLoadGatewayModeRequiresTLSURL(t *testing.T) {
	t.Setenv("HFT_GATEWAY_MODE", "on")
	t.Setenv("HFT_GATEWAY_URL", "tcp://gateway.example.com:8443")

	_, err := Load([]string{"HuggingFlowTransformers"})
	if err == nil {
		t.Fatal("expected non TLS gateway URL to fail")
	}
}

func TestLoadGatewayMode(t *testing.T) {
	t.Setenv("HFT_GATEWAY_MODE", "on")
	t.Setenv("HFT_GATEWAY_URL", "tls://gateway.example.com:8443")
	t.Setenv("HFT_UPSTREAM_DIRECT", "false")
	t.Setenv("HFT_GATEWAY_CONNECT_TIMEOUT", "7")
	t.Setenv("HFT_GATEWAY_IDLE_TIMEOUT", "90")

	cfg, err := Load([]string{"HuggingFlowTransformers"})
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if !cfg.GatewayMode {
		t.Fatal("GatewayMode should be enabled")
	}
	if cfg.GatewayURL != "tls://gateway.example.com:8443" {
		t.Fatalf("GatewayURL = %q", cfg.GatewayURL)
	}
	if cfg.UpstreamDirect {
		t.Fatal("UpstreamDirect should be false")
	}
	if cfg.GatewayConnectTimeout != 7*time.Second {
		t.Fatalf("GatewayConnectTimeout = %v", cfg.GatewayConnectTimeout)
	}
	if cfg.GatewayIdleTimeout != 90*time.Second {
		t.Fatalf("GatewayIdleTimeout = %v", cfg.GatewayIdleTimeout)
	}
}

func TestLoadGatewayServerConfig(t *testing.T) {
	t.Setenv("HFT_COORDINATION_UPSTREAM", "upstream.example.com:15566")
	t.Setenv("HFT_GATEWAY_LISTEN", "127.0.0.1:8443")
	t.Setenv("HFT_GATEWAY_TLS_CERT", "/tmp/server.crt")
	t.Setenv("HFT_GATEWAY_TLS_KEY", "/tmp/server.key")
	t.Setenv("HFT_GATEWAY_MAX_SESSIONS", "12")
	t.Setenv("HFT_GATEWAY_MAX_SESSIONS_PER_IP", "3")

	cfg, err := LoadGateway([]string{"hft-gateway"})
	if err != nil {
		t.Fatalf("LoadGateway returned error: %v", err)
	}
	if cfg.Listen != "127.0.0.1:8443" {
		t.Fatalf("Listen = %q", cfg.Listen)
	}
	if cfg.CoordinationUpstream != "upstream.example.com:15566" {
		t.Fatalf("CoordinationUpstream = %q", cfg.CoordinationUpstream)
	}
	if cfg.MaxSessions != 12 || cfg.MaxSessionsPerIP != 3 {
		t.Fatalf("limits = %d/%d", cfg.MaxSessions, cfg.MaxSessionsPerIP)
	}
}
