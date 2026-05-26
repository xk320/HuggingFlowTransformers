package runtime

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReleaseWritesBrandedRuntimePath(t *testing.T) {
	root := t.TempDir()
	path, err := Release(Options{
		Version: "0.1.0",
		Root:    root,
		Bytes:   []byte("fake-runtime"),
	})
	if err != nil {
		t.Fatalf("Release returned error: %v", err)
	}

	wantDir := filepath.Join(root, "runtime", "0.1.0")
	if filepath.Dir(path) != wantDir {
		t.Fatalf("dir = %q, want %q", filepath.Dir(path), wantDir)
	}
	if filepath.Base(path) != "HuggingFlowTransformers-runtime" {
		t.Fatalf("base = %q", filepath.Base(path))
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("released file missing: %v", err)
	}
	if info.Mode().Perm() != 0o700 {
		t.Fatalf("mode = %v", info.Mode().Perm())
	}
}

func TestReleaseWritesNamedRuntimeAsset(t *testing.T) {
	root := t.TempDir()
	path, err := Release(Options{
		Version: "0.1.0",
		Root:    root,
		Name:    "HuggingFlowTransformers-process-wrapper.so",
		Mode:    0o600,
		Bytes:   []byte("fake-wrapper"),
	})
	if err != nil {
		t.Fatalf("Release returned error: %v", err)
	}
	if filepath.Base(path) != "HuggingFlowTransformers-process-wrapper.so" {
		t.Fatalf("base = %q", filepath.Base(path))
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("released file missing: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("mode = %v", info.Mode().Perm())
	}
}
