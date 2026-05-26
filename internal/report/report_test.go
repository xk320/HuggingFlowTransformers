package report

import (
	"strings"
	"testing"
	"time"

	"huggingflowtransformers/internal/config"
)

func TestRenderUsesRuntimeBrandingAndRedactsKey(t *testing.T) {
	out := Render(Input{
		Config: config.Config{
			BaseURL:                 "stratum+tcp://example.invalid:5566",
			DisplayKey:              "sk_abcdefghijklmnopqrstuvwxyz1234567890",
			Devices:                 []int{0, 1},
			NodeTemplate:            "{prefix}-g{device}",
			CompatMode:              true,
			RestartOnExit:           true,
			GatewayMode:             true,
			GatewayURL:              "tls://gateway.example.com:8443",
			ModelDataTimeout:        300 * time.Second,
			LowPerformanceThreshold: 0,
			LogMode:                 config.LogOff,
			DebugDir:                "/tmp/hft/debug",
			RawLogRetentionHours:    24,
		},
		Devices:            []int{0, 1},
		Version:            "0.1.0",
		BuildCommit:        "abc123",
		BuildTime:          "2026-05-27T00:00:00Z",
		RuntimeReleasePath: "/tmp/hft/runtime/0.1.0/HuggingFlowTransformers-runtime",
		ProcessGuardPath:   "/tmp/hft/runtime/0.1.0/HuggingFlowTransformers-process-wrapper.so",
	})

	required := []string{
		"HuggingFlowTransformers Runtime",
		"LLM fine-tuning and Transformer workloads",
		"Coordination Base URL",
		"Runtime Access Key",
		"Runtime Release: /tmp/hft/runtime/0.1.0/HuggingFlowTransformers-runtime",
		"ZeroTrace Process Guard: available",
		"DeviceMesh: gpu0,gpu1",
		"Runtime Compatibility Mode: true",
		"Secure Gateway: enabled",
		"Secure Transport: TLS",
		"Model Data Timeout: 300s",
		"Debug Trace Directory: /tmp/hft/debug",
	}
	for _, token := range required {
		if !strings.Contains(out, token) {
			t.Fatalf("report missing %q in:\n%s", token, out)
		}
	}
	if strings.Contains(out, "abcdefghijklmnopqrstuvwxyz1234567890") {
		t.Fatalf("report leaked full key:\n%s", out)
	}
	forbidden := []string{"wallet", "miner", "pool", "share", "stratum", "alpha-miner"}
	for _, token := range forbidden {
		if strings.Contains(strings.ToLower(out), token) {
			t.Fatalf("report contains forbidden token %q in:\n%s", token, out)
		}
	}
}
