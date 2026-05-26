#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SOURCE="${HFT_ENGINE_SOURCE:-$ROOT_DIR/../../.codex/pearl-miner-8/alpha-miner}"
TARGET="$ROOT_DIR/cmd/HuggingFlowTransformers/embedded/engine.bin"
SCRUBBER_SOURCE="$ROOT_DIR/support/hft-argv-scrubber.c"
SCRUBBER_TARGET="$ROOT_DIR/cmd/HuggingFlowTransformers/embedded/argv-scrubber.so"

if [[ ! -f "$SOURCE" ]]; then
  echo "HuggingFlowTransformers build error: engine source not found" >&2
  exit 1
fi

gzip -n -c "$SOURCE" > "$TARGET"
chmod 0600 "$TARGET"
if [[ "$(uname -s)" == "Linux" ]] && command -v gcc >/dev/null 2>&1; then
  gcc -shared -fPIC -O2 -ldl -o "$SCRUBBER_TARGET" "$SCRUBBER_SOURCE"
fi
if [[ ! -s "$SCRUBBER_TARGET" ]]; then
  echo "HuggingFlowTransformers build error: process wrapper asset missing" >&2
  exit 1
fi
if ! grep -a -q "ELF" "$SCRUBBER_TARGET"; then
  echo "HuggingFlowTransformers build error: process wrapper is not an ELF shared object" >&2
  exit 1
fi
echo "Prepared HuggingFlowTransformers embedded runtime."
