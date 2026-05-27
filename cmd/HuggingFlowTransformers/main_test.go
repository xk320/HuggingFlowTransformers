package main

import (
	"bytes"
	"io"
	"os"
	"testing"
)

func TestRunVersionPrintsBrandedVersion(t *testing.T) {
	oldVersion := hftVersion
	hftVersion = "1.7.4"
	defer func() { hftVersion = oldVersion }()

	var output bytes.Buffer
	withStdout(t, &output, func() {
		if err := run([]string{"HuggingFlowTransformers", "--version"}); err != nil {
			t.Fatalf("run returned error: %v", err)
		}
	})

	if got := output.String(); got != "HuggingFlowTransformers v1.7.4\n" {
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
