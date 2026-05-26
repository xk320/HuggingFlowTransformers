#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

for script in install-client.sh install-gateway.sh; do
  path="$ROOT_DIR/scripts/$script"
  test -x "$path"
  bash -n "$path"
done

grep -q "HFT_GATEWAY_CA_URL" "$ROOT_DIR/scripts/install-client.sh"
grep -q "download_gateway_ca_from_gateway" "$ROOT_DIR/scripts/install-client.sh"
grep -q "openssl s_client" "$ROOT_DIR/scripts/install-client.sh"
grep -q "HFT_PACKAGE_URL" "$ROOT_DIR/scripts/install-client.sh"
grep -q "systemctl enable" "$ROOT_DIR/scripts/install-client.sh"
grep -q "HFT_GATEWAY_TLS_CERT_URL" "$ROOT_DIR/scripts/install-gateway.sh"
grep -q "HFT_COORDINATION_UPSTREAM" "$ROOT_DIR/scripts/install-gateway.sh"
grep -q "systemctl enable" "$ROOT_DIR/scripts/install-gateway.sh"
