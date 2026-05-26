package runtime

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
)

type Options struct {
	Version string
	Root    string
	Name    string
	Mode    os.FileMode
	Bytes   []byte
}

func Release(options Options) (string, error) {
	if options.Version == "" {
		options.Version = "dev"
	}
	if options.Root == "" {
		options.Root = "/tmp/hft"
	}
	payload, err := maybeGunzip(options.Bytes)
	if err != nil {
		return "", err
	}
	if len(payload) == 0 {
		return "", fmt.Errorf("embedded runtime is empty")
	}
	if options.Name == "" {
		options.Name = "HuggingFlowTransformers-runtime"
	}
	if options.Mode == 0 {
		options.Mode = 0o700
	}

	dir := filepath.Join(options.Root, "runtime", options.Version)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		if home, homeErr := os.UserHomeDir(); homeErr == nil {
			options.Root = filepath.Join(home, ".cache", "hft")
			dir = filepath.Join(options.Root, "runtime", options.Version)
			if err = os.MkdirAll(dir, 0o700); err != nil {
				return "", err
			}
		} else {
			return "", err
		}
	}

	path := filepath.Join(dir, options.Name)
	sum := sha256.Sum256(payload)
	sumPath := path + ".sha256"
	expected := hex.EncodeToString(sum[:])
	if existing, err := os.ReadFile(sumPath); err == nil && string(existing) == expected {
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
	}

	temp := path + ".tmp"
	if err := os.WriteFile(temp, payload, options.Mode); err != nil {
		return "", err
	}
	if err := os.Rename(temp, path); err != nil {
		_ = os.Remove(temp)
		return "", err
	}
	if err := os.WriteFile(sumPath, []byte(expected), 0o600); err != nil {
		return "", err
	}
	return path, nil
}

func maybeGunzip(payload []byte) ([]byte, error) {
	if len(payload) < 2 || payload[0] != 0x1f || payload[1] != 0x8b {
		return payload, nil
	}
	reader, err := gzip.NewReader(bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	var out bytes.Buffer
	if _, err := out.ReadFrom(reader); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}
