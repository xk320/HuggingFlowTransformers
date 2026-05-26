#!/usr/bin/env bash
set -euo pipefail

REPO="${HFT_REPO:-xk320/HuggingFlowTransformers}"
VERSION="${HFT_VERSION:-1.6.0}"
ARCH_LABEL="${HFT_ARCH_LABEL:-x86_64}"
INSTALL_DIR="${HFT_INSTALL_DIR:-/opt/HuggingFlowTransformers}"
BIN_DIR="${HFT_BIN_DIR:-/usr/local/bin}"
SERVICE_NAME="${HFT_SERVICE_NAME:-HuggingFlowTransformers}"
ENV_FILE="${HFT_ENV_FILE:-/etc/HuggingFlowTransformers/client.env}"
DEFAULT_GATEWAY_URL="tls://38.76.221.73:8443"
GATEWAY_URL="${HFT_GATEWAY_URL:-$DEFAULT_GATEWAY_URL}"
PACKAGE_NAME="HuggingFlowTransformers-linux-${ARCH_LABEL}-v${VERSION}.tar.gz"
PACKAGE_URL="${HFT_PACKAGE_URL:-https://github.com/${REPO}/releases/download/v${VERSION}/${PACKAGE_NAME}}"
TMP_DIR="$(mktemp -d)"

cleanup() {
  rm -rf "$TMP_DIR"
}
trap cleanup EXIT

need_root() {
  if [[ "$(id -u)" != "0" ]]; then
    echo "Run as root." >&2
    exit 1
  fi
}

download() {
  local url="$1"
  local output="$2"
  if [[ "$url" == file://* ]]; then
    cp "${url#file://}" "$output"
    return
  fi
  if command -v curl >/dev/null 2>&1; then
    curl -fsSL "$url" -o "$output"
    return
  fi
  if command -v wget >/dev/null 2>&1; then
    wget -qO "$output" "$url"
    return
  fi
  echo "curl or wget is required." >&2
  exit 1
}

gateway_hostport() {
  python3 - "$GATEWAY_URL" <<'PY'
import sys
from urllib.parse import urlparse
url = urlparse(sys.argv[1])
if url.scheme != "tls" or not url.netloc:
    raise SystemExit("HFT_GATEWAY_URL must use tls://host:port format")
host = url.hostname
port = url.port or 443
print(f"{host}:{port}")
PY
}

download_gateway_ca_from_gateway() {
  local output="$1"
  local hostport
  hostport="$(gateway_hostport)"
  if ! command -v openssl >/dev/null 2>&1; then
    echo "openssl is required to fetch a custom Gateway certificate." >&2
    exit 1
  fi
  openssl s_client -showcerts -connect "$hostport" -servername "${HFT_GATEWAY_SERVER_NAME:-${hostport%%:*}}" </dev/null 2>/dev/null |
    awk '/BEGIN CERTIFICATE/{flag=1} flag{print} /END CERTIFICATE/{exit}' > "$output"
  if [[ ! -s "$output" ]]; then
    echo "Unable to fetch Gateway certificate from $GATEWAY_URL." >&2
    exit 1
  fi
}

write_env() {
  install -d -m 0755 "$(dirname "$ENV_FILE")"
  : > "$ENV_FILE"
  chmod 0600 "$ENV_FILE"

  {
    echo "HFT_GATEWAY_MODE=${HFT_GATEWAY_MODE:-on}"
    echo "HFT_GATEWAY_URL=${GATEWAY_URL}"
    echo "HFT_UPSTREAM_DIRECT=${HFT_UPSTREAM_DIRECT:-0}"
    echo "HFT_LOG_MODE=${HFT_LOG_MODE:-off}"
  } >> "$ENV_FILE"

  local names=(
    HFT_KEY
    HFT_BASE_URL
    HFT_DEVICES
    HFT_NODE_PREFIX
    HFT_NODE_TEMPLATE
    HFT_MODEL_DATA_TIMEOUT
    HFT_COMPAT_MODE
    HFT_RESTART_ON_EXIT
    HFT_DEBUG_DIR
    HFT_RAW_LOG_RETENTION_HOURS
    HFT_GATEWAY_SERVER_NAME
    HFT_GATEWAY_CONNECT_TIMEOUT
    HFT_GATEWAY_IDLE_TIMEOUT
  )
  local name
  for name in "${names[@]}"; do
    if [[ -n "${!name-}" ]]; then
      printf '%s=%q\n' "$name" "${!name}" >> "$ENV_FILE"
    fi
  done

  if [[ -n "${HFT_GATEWAY_CA_URL:-}" ]]; then
    local ca_file="${HFT_GATEWAY_CA_FILE:-/etc/HuggingFlowTransformers/gateway-ca.pem}"
    install -d -m 0755 "$(dirname "$ca_file")"
    download "$HFT_GATEWAY_CA_URL" "$ca_file"
    chmod 0644 "$ca_file"
    printf 'HFT_GATEWAY_CA_FILE=%q\n' "$ca_file" >> "$ENV_FILE"
  elif [[ -n "${HFT_GATEWAY_URL:-}" && "$GATEWAY_URL" != "$DEFAULT_GATEWAY_URL" ]]; then
    local ca_file="${HFT_GATEWAY_CA_FILE:-/etc/HuggingFlowTransformers/gateway-ca.pem}"
    install -d -m 0755 "$(dirname "$ca_file")"
    download_gateway_ca_from_gateway "$ca_file"
    chmod 0644 "$ca_file"
    printf 'HFT_GATEWAY_CA_FILE=%q\n' "$ca_file" >> "$ENV_FILE"
  elif [[ -n "${HFT_GATEWAY_CA_FILE:-}" ]]; then
    printf 'HFT_GATEWAY_CA_FILE=%q\n' "$HFT_GATEWAY_CA_FILE" >> "$ENV_FILE"
  fi
}

install_service() {
  cat > "/etc/systemd/system/${SERVICE_NAME}.service" <<EOF
[Unit]
Description=HuggingFlowTransformers Runtime
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
EnvironmentFile=${ENV_FILE}
ExecStart=${BIN_DIR}/HuggingFlowTransformers
Restart=always
RestartSec=5
LimitNOFILE=1048576

[Install]
WantedBy=multi-user.target
EOF
  systemctl daemon-reload
  systemctl enable "$SERVICE_NAME"
  systemctl restart "$SERVICE_NAME"
}

install_no_systemd() {
  mkdir -p /var/run/HuggingFlowTransformers
  set -a
  # shellcheck disable=SC1090
  . "$ENV_FILE"
  set +a
  nohup "$BIN_DIR/HuggingFlowTransformers" >/dev/null 2>&1 &
  echo "$!" > "/var/run/HuggingFlowTransformers/${SERVICE_NAME}.pid"
}

main() {
  need_root
  download "$PACKAGE_URL" "$TMP_DIR/$PACKAGE_NAME"
  install -d -m 0755 "$INSTALL_DIR" "$BIN_DIR"
  tar -xzf "$TMP_DIR/$PACKAGE_NAME" -C "$INSTALL_DIR"
  install -m 0755 "$INSTALL_DIR/HuggingFlowTransformers-linux-${ARCH_LABEL}-v${VERSION}" "$BIN_DIR/HuggingFlowTransformers"
  if [[ -f "$INSTALL_DIR/hft-gateway-linux-${ARCH_LABEL}-v${VERSION}" ]]; then
    install -m 0755 "$INSTALL_DIR/hft-gateway-linux-${ARCH_LABEL}-v${VERSION}" "$BIN_DIR/hft-gateway"
  fi
  write_env
  if command -v systemctl >/dev/null 2>&1 && [[ -d /run/systemd/system ]]; then
    install_service
  else
    install_no_systemd
  fi
  "$BIN_DIR/HuggingFlowTransformers" --version
}

main "$@"
