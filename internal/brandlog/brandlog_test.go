package brandlog

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

func TestStdoutModePrintsOnlyBrandedEvents(t *testing.T) {
	var out bytes.Buffer
	logger := New(Options{Mode: ModeStdout, Version: "0.1.0", Writer: &out})

	logger.Event(Event{
		Time:   time.Date(2026, 5, 26, 21, 10, 0, 0, time.UTC),
		Level:  "info",
		Device: 0,
		Node:   "host-g0",
		Name:   "model_data_submitted",
	})

	got := out.String()
	if !strings.Contains(got, "component=hft") || !strings.Contains(got, "event=model_data_submitted") {
		t.Fatalf("missing branded event fields: %q", got)
	}
	forbidden := []string{"alpha-miner", "--pool", "--address", "wallet"}
	for _, token := range forbidden {
		if strings.Contains(got, token) {
			t.Fatalf("stdout leaked forbidden token %q in %q", token, got)
		}
	}
}

func TestOffModeIsQuiet(t *testing.T) {
	var out bytes.Buffer
	logger := New(Options{Mode: ModeOff, Version: "0.1.0", Writer: &out})

	logger.Event(Event{Name: "started", Level: "info"})

	if out.Len() != 0 {
		t.Fatalf("off mode wrote %q", out.String())
	}
}

func TestRedactsKeyLikeValues(t *testing.T) {
	if got := Redact("sk_abcdefghijklmnopqrstuvwxyz"); strings.Contains(got, "abcdefghijklmnopqrstuvwxyz") {
		t.Fatalf("Redact leaked full key: %q", got)
	}
	if got := Redact("prl1pabcdefghijklmnopqrstuvwxyz"); strings.Contains(got, "abcdefghijklmnopqrstuvwxyz") {
		t.Fatalf("Redact leaked internal key: %q", got)
	}
}
