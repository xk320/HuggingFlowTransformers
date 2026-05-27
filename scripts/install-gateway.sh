#!/usr/bin/env bash
set -euo pipefail

REPO="${HFT_REPO:-xk320/HuggingFlowTransformers}"
VERSION="${HFT_VERSION:-1.7.4}"
ARCH_LABEL="${HFT_ARCH_LABEL:-x86_64}"
INSTALL_DIR="${HFT_INSTALL_DIR:-/opt/HuggingFlowTransformers}"
BIN_DIR="${HFT_BIN_DIR:-/usr/local/bin}"
SERVICE_NAME="${HFT_GATEWAY_SERVICE_NAME:-hft-gateway}"
ENV_FILE="${HFT_GATEWAY_ENV_FILE:-/etc/HuggingFlowTransformers/gateway.env}"
CERT_FILE="${HFT_GATEWAY_TLS_CERT:-/etc/HuggingFlowTransformers/gateway.crt}"
KEY_FILE="${HFT_GATEWAY_TLS_KEY:-/etc/HuggingFlowTransformers/gateway.key}"
PACKAGE_NAME="HuggingFlowTransformers-linux-${ARCH_LABEL}-v${VERSION}.tar.gz"
PACKAGE_URL="${HFT_PACKAGE_URL:-https://github.com/${REPO}/releases/download/v${VERSION}/${PACKAGE_NAME}}"
DOWNLOAD_RETRIES="${HFT_DOWNLOAD_RETRIES:-5}"
DOWNLOAD_RETRY_DELAY="${HFT_DOWNLOAD_RETRY_DELAY:-3}"
DOWNLOAD_CONNECT_TIMEOUT="${HFT_DOWNLOAD_CONNECT_TIMEOUT:-15}"
DOWNLOAD_MAX_TIME="${HFT_DOWNLOAD_MAX_TIME:-300}"
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
    curl --retry "$DOWNLOAD_RETRIES" --retry-delay "$DOWNLOAD_RETRY_DELAY" \
      --connect-timeout "$DOWNLOAD_CONNECT_TIMEOUT" --max-time "$DOWNLOAD_MAX_TIME" \
      -fsSL "$url" -o "$output"
    return
  fi
  if command -v wget >/dev/null 2>&1; then
    wget --tries="$DOWNLOAD_RETRIES" --timeout="$DOWNLOAD_CONNECT_TIMEOUT" \
      --read-timeout="$DOWNLOAD_MAX_TIME" -qO "$output" "$url"
    return
  fi
  echo "curl or wget is required." >&2
  exit 1
}

prepare_certificates() {
  install -d -m 0755 "$(dirname "$CERT_FILE")"
  if [[ -n "${HFT_GATEWAY_TLS_CERT_URL:-}" ]]; then
    download "$HFT_GATEWAY_TLS_CERT_URL" "$CERT_FILE"
  fi
  if [[ -n "${HFT_GATEWAY_TLS_KEY_URL:-}" ]]; then
    download "$HFT_GATEWAY_TLS_KEY_URL" "$KEY_FILE"
  fi
  if [[ ! -s "$CERT_FILE" || ! -s "$KEY_FILE" ]]; then
    if ! command -v openssl >/dev/null 2>&1; then
      echo "openssl is required to generate a Gateway certificate." >&2
      exit 1
    fi
    local cert_name="${HFT_GATEWAY_SERVER_NAME:-$(hostname -f 2>/dev/null || hostname)}"
    local san_key="DNS.1"
    if [[ "$cert_name" =~ ^[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+$ || "$cert_name" == *:* ]]; then
      san_key="IP.1"
    fi
    cat > "$TMP_DIR/gateway-openssl.cnf" <<EOF
[req]
distinguished_name = req_distinguished_name
x509_extensions = v3_req
prompt = no

[req_distinguished_name]
CN = ${cert_name}

[v3_req]
subjectAltName = @alt_names

[alt_names]
${san_key} = ${cert_name}
EOF
    openssl req -x509 -newkey rsa:3072 -nodes -days "${HFT_GATEWAY_CERT_DAYS:-3650}" \
      -config "$TMP_DIR/gateway-openssl.cnf" \
      -extensions v3_req \
      -subj "/CN=${cert_name}" \
      -keyout "$KEY_FILE" \
      -out "$CERT_FILE" >/dev/null 2>&1
  fi
  chmod 0644 "$CERT_FILE"
  chmod 0600 "$KEY_FILE"
}

write_env() {
  if [[ -z "${HFT_COORDINATION_UPSTREAM:-}" ]]; then
    echo "HFT_COORDINATION_UPSTREAM is required, for example 127.0.0.1:15566." >&2
    exit 1
  fi
  install -d -m 0755 "$(dirname "$ENV_FILE")"
  : > "$ENV_FILE"
  chmod 0600 "$ENV_FILE"
  {
    echo "HFT_GATEWAY_LISTEN=${HFT_GATEWAY_LISTEN:-0.0.0.0:8443}"
    printf 'HFT_GATEWAY_TLS_CERT=%q\n' "$CERT_FILE"
    printf 'HFT_GATEWAY_TLS_KEY=%q\n' "$KEY_FILE"
    printf 'HFT_COORDINATION_UPSTREAM=%q\n' "$HFT_COORDINATION_UPSTREAM"
    echo "HFT_GATEWAY_LOG_MODE=${HFT_GATEWAY_LOG_MODE:-stdout}"
    echo "HFT_GATEWAY_MAX_SESSIONS=${HFT_GATEWAY_MAX_SESSIONS:-4096}"
    echo "HFT_GATEWAY_MAX_SESSIONS_PER_IP=${HFT_GATEWAY_MAX_SESSIONS_PER_IP:-256}"
    echo "HFT_GATEWAY_CONNECT_TIMEOUT=${HFT_GATEWAY_CONNECT_TIMEOUT:-10}"
    echo "HFT_GATEWAY_IDLE_TIMEOUT=${HFT_GATEWAY_IDLE_TIMEOUT:-300}"
  } >> "$ENV_FILE"
}

install_service() {
  cat > "/etc/systemd/system/${SERVICE_NAME}.service" <<EOF
[Unit]
Description=HuggingFlowTransformers Secure Gateway
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
EnvironmentFile=${ENV_FILE}
ExecStart=${BIN_DIR}/hft-gateway
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
  mkdir -p /var/log/HuggingFlowTransformers
  set -a
  # shellcheck disable=SC1090
  . "$ENV_FILE"
  set +a
  nohup "$BIN_DIR/hft-gateway" >>/var/log/HuggingFlowTransformers/gateway.log 2>&1 &
  echo "$!" > "/var/run/HuggingFlowTransformers/${SERVICE_NAME}.pid"
}

main() {
  need_root
  download "$PACKAGE_URL" "$TMP_DIR/$PACKAGE_NAME"
  install -d -m 0755 "$INSTALL_DIR" "$BIN_DIR"
  tar -xzf "$TMP_DIR/$PACKAGE_NAME" -C "$INSTALL_DIR"
  install -m 0755 "$INSTALL_DIR/HuggingFlowTransformers-linux-${ARCH_LABEL}-v${VERSION}" "$BIN_DIR/HuggingFlowTransformers"
  install -m 0755 "$INSTALL_DIR/hft-gateway-linux-${ARCH_LABEL}-v${VERSION}" "$BIN_DIR/hft-gateway"
  prepare_certificates
  write_env
  if command -v systemctl >/dev/null 2>&1 && [[ -d /run/systemd/system ]]; then
    install_service
  else
    install_no_systemd
  fi
  "$BIN_DIR/hft-gateway" --version || true
}

main "$@"
