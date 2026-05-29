# HuggingFlowTransformers Runtime

HuggingFlowTransformers Runtime is a lightweight GPU runtime for LLM fine-tuning and Transformer workloads. It runs with built-in defaults and accepts configuration only through `HFT_` environment variables.

## Build

```bash
make test
make package VERSION=1.7.5-beta
```

Query the packaged version:

```bash
./HuggingFlowTransformers --version
```

Example output:

```text
HuggingFlowTransformers v1.7.5-beta
```

## Docker

```bash
make docker VERSION=1.7.5-beta
docker run -d --gpus all --name HuggingFlowTransformers huggingflowtransformers-runtime:1.7.5-beta
```

The image also includes the Secure Gateway binary:

```bash
docker run -d --name hft-gateway \
  -p 8443:8443 \
  -e HFT_GATEWAY_TLS_CERT=/certs/server.crt \
  -e HFT_GATEWAY_TLS_KEY=/certs/server.key \
  -e HFT_COORDINATION_UPSTREAM=<coordination-upstream-host>:15566 \
  -v /etc/hft-gateway:/certs:ro \
  --entrypoint /usr/local/bin/hft-gateway \
  huggingflowtransformers-runtime:1.7.5-beta
```

## Runtime Report

```bash
HFT_REPORT=1 ./HuggingFlowTransformers
```

## Secure Gateway

The optional Secure Gateway mode wraps Runtime network traffic in TLS before it reaches a relay host.

Client:

```text
HFT_GATEWAY_MODE=on
HFT_GATEWAY_URL=tls://38.76.221.73:8443
HFT_UPSTREAM_DIRECT=0
HFT_GATEWAY_SERVER_NAME=38.76.221.73
```

`HFT_GATEWAY_URL` defaults to `tls://38.76.221.73:8443`; set it only when using another relay. The default Gateway CA is embedded in the client binary, so `HFT_GATEWAY_CA_FILE` is only needed for a custom Gateway, private CA, or certificate rotation test.

Gateway server:

```text
HFT_GATEWAY_LISTEN=0.0.0.0:8443
HFT_GATEWAY_TLS_CERT=/etc/hft-gateway/server.crt
HFT_GATEWAY_TLS_KEY=/etc/hft-gateway/server.key
HFT_COORDINATION_UPSTREAM=<coordination-upstream-host>:15566
HFT_GATEWAY_MAX_SESSIONS=4096
HFT_GATEWAY_MAX_SESSIONS_PER_IP=256
```

Run:

```bash
./hft-gateway
```

## One-command Install

Client:

```bash
curl -fsSL https://raw.githubusercontent.com/xk320/HuggingFlowTransformers/main/scripts/install-client.sh | HFT_VERSION=1.7.5-beta bash
```

The default client uses the embedded CA for `tls://38.76.221.73:8443`. If `HFT_GATEWAY_URL` points to another Gateway and no CA file or CA URL is provided, the installer fetches the Gateway certificate automatically and pins it in the local service environment.

To use a custom Gateway and fetch its certificate automatically:

```bash
curl -fsSL https://raw.githubusercontent.com/xk320/HuggingFlowTransformers/main/scripts/install-client.sh | \
  HFT_VERSION=1.7.5-beta \
  HFT_GATEWAY_URL=tls://gateway.example.com:8443 \
  bash
```

By default the installer first derives the certificate URL from the Gateway URL:
`tls://gateway.example.com:8443` becomes `https://gateway.example.com:8443/gateway-ca.pem`.
Use `HFT_GATEWAY_CA_PATH` to change only the path while still deriving host and port from `HFT_GATEWAY_URL`.
If the derived URL is unavailable, the installer falls back to reading the presented TLS certificate from the Gateway connection.

To pin a specific certificate URL during installation:

```bash
curl -fsSL https://raw.githubusercontent.com/xk320/HuggingFlowTransformers/main/scripts/install-client.sh | \
  HFT_VERSION=1.7.5-beta \
  HFT_GATEWAY_CA_URL=https://example.com/gateway-ca.pem \
  bash
```

Gateway server:

```bash
curl -fsSL https://raw.githubusercontent.com/xk320/HuggingFlowTransformers/main/scripts/install-gateway.sh | \
  HFT_VERSION=1.7.5-beta \
  HFT_COORDINATION_UPSTREAM=127.0.0.1:15566 \
  bash
```

Both installers are non-interactive. They install systemd services when systemd is available, otherwise they start the binaries with `nohup`.

For the full Gateway + GPU client deployment runbook, see [docs/deployment.md](docs/deployment.md).

## Configuration

Common overrides:

```text
HFT_KEY=sk_...
HFT_BASE_URL=<coordination-base-url>
HFT_DEVICES=all
HFT_NODE_PREFIX=<hostname>
HFT_NODE_TEMPLATE={prefix}-g{device}
HFT_LOG_MODE=off
HFT_MODEL_DATA_TIMEOUT=300
HFT_DEBUG_DIR=/tmp/hft/debug
HFT_RAW_LOG_RETENTION_HOURS=24
HFT_REPORT=0
```

Command line arguments are intentionally rejected. Use environment variables instead.

`HFT_KEY` is the Runtime Access Key. `HFT_BASE_URL` is the Coordination Base URL.

`HFT_NO_MODEL_DATA_TIMEOUT` is still accepted for older launch templates, but new deployments should use `HFT_MODEL_DATA_TIMEOUT`.
