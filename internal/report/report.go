package report

import (
	"fmt"
	"strings"

	"huggingflowtransformers/internal/brandlog"
	"huggingflowtransformers/internal/config"
)

type Input struct {
	Config             config.Config
	Devices            []int
	Version            string
	BuildCommit        string
	BuildTime          string
	RuntimeReleasePath string
	ProcessGuardPath   string
}

func Render(input Input) string {
	cfg := input.Config
	var b strings.Builder
	version := input.Version
	if version == "" {
		version = "dev"
	}
	buildCommit := input.BuildCommit
	if buildCommit == "" {
		buildCommit = "unknown"
	}
	buildTime := input.BuildTime
	if buildTime == "" {
		buildTime = "unknown"
	}

	fmt.Fprintln(&b, "HuggingFlowTransformers Runtime")
	fmt.Fprintln(&b, "A lightweight GPU runtime for LLM fine-tuning and Transformer workloads.")
	fmt.Fprintf(&b, "Version: %s\n", version)
	fmt.Fprintf(&b, "Build Commit: %s\n", buildCommit)
	fmt.Fprintf(&b, "Build Time: %s\n", buildTime)
	fmt.Fprintf(&b, "Runtime Release: %s\n", valueOrUnavailable(input.RuntimeReleasePath))
	fmt.Fprintf(&b, "ZeroTrace Process Guard: %s\n", guardStatus(input.ProcessGuardPath))
	fmt.Fprintf(&b, "Coordination Base URL: %s\n", redactedEndpoint(cfg.BaseURL))
	fmt.Fprintf(&b, "Runtime Access Key: %s\n", brandlog.Redact(cfg.DisplayKey))
	fmt.Fprintf(&b, "DeviceMesh: %s\n", formatDevices(input.Devices))
	fmt.Fprintf(&b, "Node Template: %s\n", cfg.NodeTemplate)
	fmt.Fprintf(&b, "Runtime Compatibility Mode: %t\n", cfg.CompatMode)
	fmt.Fprintf(&b, "Runtime Restart Guard: %t\n", cfg.RestartOnExit)
	fmt.Fprintf(&b, "Secure Gateway: %s\n", gatewayStatus(cfg))
	fmt.Fprintf(&b, "Secure Transport: %s\n", secureTransport(cfg))
	fmt.Fprintf(&b, "Model Data Timeout: %.0fs\n", cfg.ModelDataTimeout.Seconds())
	fmt.Fprintf(&b, "Low Performance Threshold: %d\n", cfg.LowPerformanceThreshold)
	fmt.Fprintf(&b, "Log Mode: %s\n", cfg.LogMode)
	fmt.Fprintf(&b, "Debug Trace Directory: %s\n", cfg.DebugDir)
	fmt.Fprintf(&b, "Debug Trace Retention: %dh\n", cfg.RawLogRetentionHours)
	return b.String()
}

func gatewayStatus(cfg config.Config) string {
	if !cfg.GatewayMode {
		return "disabled"
	}
	return "enabled"
}

func secureTransport(cfg config.Config) string {
	if !cfg.GatewayMode {
		return "direct"
	}
	return "TLS"
}

func valueOrUnavailable(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "<unavailable>"
	}
	return value
}

func guardStatus(path string) string {
	if strings.TrimSpace(path) == "" {
		return "unavailable"
	}
	return "available"
}

func formatDevices(devices []int) string {
	if len(devices) == 0 {
		return "auto"
	}
	parts := make([]string, 0, len(devices))
	for _, device := range devices {
		parts = append(parts, fmt.Sprintf("gpu%d", device))
	}
	return strings.Join(parts, ",")
}

func redactedEndpoint(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "<unset>"
	}
	return "<configured>"
}
