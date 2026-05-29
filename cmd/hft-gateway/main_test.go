package main

import (
	"bytes"
	"io"
	"os"
	"testing"
)

func TestRunVersionPrintsGatewayVersion(t *testing.T) {
	oldVersion := gatewayVersion
	gatewayVersion = "1.7.5-beta"
	defer func() { gatewayVersion = oldVersion }()

	var output bytes.Buffer
	withStdout(t, &output, func() {
		if err := run([]string{"hft-gateway", "--version"}); err != nil {
			t.Fatalf("run returned error: %v", err)
		}
	})

	if got := output.String(); got != "HuggingFlowTransformers Gateway v1.7.5-beta\n" {
		t.Fatalf("version output = %q", got)
	}
}

func withStdout(t *testing.T, writer io.Writer, fn func()) {
	t.Helper()
	old := os.Stdout
	read, write, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdout = write
	defer func() { os.Stdout = old }()

	fn()
	_ = write.Close()
	if _, err := io.Copy(writer, read); err != nil {
		t.Fatalf("read stdout: %v", err)
	}
}
