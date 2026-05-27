# HuggingFlowTransformers 安装实施部署文档

本文档说明 HuggingFlowTransformers 客户端与 Gateway 的生产部署方式、自动化安装命令、验证标准、清理步骤和常见故障处理。文档不包含任何密码、私钥、账户密钥或代理凭据。

## 1. 架构

HuggingFlowTransformers 采用“客户端 + Gateway”的两段式链路：

```text
GPU 客户端
  -> HuggingFlowTransformers TLS 封装
  -> Gateway
  -> 上游协调服务 / 中转端口
```

职责划分：

- GPU 客户端：安装 `HuggingFlowTransformers`，默认以 `<hostname>-g0`、`<hostname>-g1` 形式生成节点名。
- Gateway：安装 `hft-gateway`，监听 TLS 端口，解密后转发到上游协调服务。
- 上游协调服务：通常是本机已有的中转端口，例如 `127.0.0.1:15566`。

## 2. 前置条件

Gateway 服务器：

- root 权限。
- 能访问 GitHub release 下载地址。
- 已有可用上游端口，例如本机 `127.0.0.1:15566`。
- 安装 `curl` 或 `wget`，安装 `openssl`。
- 推荐 systemd；无 systemd 时脚本会退化为 nohup 运行。

GPU 客户端：

- root 权限。
- NVIDIA 驱动可用，`nvidia-smi` 能看到目标 GPU。
- 能访问 GitHub release 下载地址。
- 能连接 Gateway 的 TLS 端口。
- 安装 `curl` 或 `wget`，安装 `openssl`。

## 3. 版本与产物

当前稳定版：

- 版本：`v1.7.4`
- Release 仓库：`https://github.com/xk320/HuggingFlowTransformers`
- Linux x86_64 包名：`HuggingFlowTransformers-linux-x86_64-v1.7.4.tar.gz`

查询版本：

```bash
HuggingFlowTransformers --version
hft-gateway --version
```

预期输出示例：

```text
HuggingFlowTransformers v1.7.4
HuggingFlowTransformers Gateway v1.7.4
```

## 4. Gateway 一键部署

在中转服务器执行：

```bash
curl --retry 3 --retry-delay 3 --connect-timeout 15 --max-time 120 -fsSL \
  https://raw.githubusercontent.com/xk320/HuggingFlowTransformers/main/scripts/install-gateway.sh | \
  HFT_VERSION=1.7.4 \
  HFT_COORDINATION_UPSTREAM=127.0.0.1:15566 \
  HFT_GATEWAY_LISTEN=0.0.0.0:8443 \
  HFT_GATEWAY_SERVER_NAME=<gateway-ip-or-domain> \
  bash
```

关键变量：

- `HFT_COORDINATION_UPSTREAM`：Gateway 解密后转发到的上游地址，必填。
- `HFT_GATEWAY_LISTEN`：Gateway 监听地址，默认建议 `0.0.0.0:8443`。
- `HFT_GATEWAY_SERVER_NAME`：证书 SAN 使用的 IP 或域名。客户端用 IP 连接时必须填写该 IP。
- `HFT_VERSION`：安装版本，当前稳定版为 `1.7.4`。

证书规则：

- 如果提供 `HFT_GATEWAY_TLS_CERT_URL` 和 `HFT_GATEWAY_TLS_KEY_URL`，脚本会下载指定证书和私钥。
- 如果未提供证书，脚本会自动生成自签证书。
- 自动生成证书会写入 Subject Alternative Name；`HFT_GATEWAY_SERVER_NAME` 是 IP 时写入 `IP.1`，是域名时写入 `DNS.1`。
- 证书路径：`/etc/HuggingFlowTransformers/gateway.crt`
- 私钥路径：`/etc/HuggingFlowTransformers/gateway.key`

验证 Gateway：

```bash
/usr/local/bin/hft-gateway --version
systemctl is-active hft-gateway 2>/dev/null || true
ss -lntp | grep ':8443'
openssl x509 -in /etc/HuggingFlowTransformers/gateway.crt -noout -subject -ext subjectAltName
journalctl -u hft-gateway --since '10 minutes ago' --no-pager | tail -n 80
```

通过标准：

- `hft-gateway --version` 输出版本号。
- systemd 环境下 `systemctl is-active hft-gateway` 输出 `active`。
- `ss` 能看到 Gateway 监听端口。
- 证书存在 `Subject Alternative Name`。

## 5. GPU 客户端一键部署

指定 Gateway 安装：

```bash
curl --retry 3 --retry-delay 3 --connect-timeout 15 --max-time 120 -fsSL \
  https://raw.githubusercontent.com/xk320/HuggingFlowTransformers/main/scripts/install-client.sh | \
  HFT_VERSION=1.7.4 \
  HFT_GATEWAY_URL=tls://<gateway-ip-or-domain>:8443 \
  HFT_DEVICES=0 \
  bash
```

常用变量：

- `HFT_GATEWAY_URL`：指定 Gateway 地址。只要指定的地址不同于内置默认 Gateway，安装脚本会自动从该 Gateway 拉取证书。
- `HFT_DEVICES`：GPU 列表，例如 `0` 或 `0,1`。
- `HFT_LOG_MODE`：默认 `off`。调试时可设为 `stdout`，方便从日志确认提交。
- `HFT_MODEL_DATA_TIMEOUT`：无提交超时阈值，调试时可设为 `180`。
- `HFT_DOWNLOAD_MAX_TIME`：下载安装包最大耗时，默认 `300` 秒。
- `HFT_DOWNLOAD_RETRIES`：下载重试次数，默认 `5`。
- `HFT_DOWNLOAD_CONNECT_TIMEOUT`：下载连接超时，默认 `15` 秒。

自动证书下载规则：

- 指定 `HFT_GATEWAY_URL=tls://<custom-gateway>:8443` 时，脚本会用 `openssl s_client` 连接 Gateway。
- 脚本提取 Gateway 证书并保存到 `/etc/HuggingFlowTransformers/gateway-ca.pem`。
- 脚本会在 `/etc/HuggingFlowTransformers/client.env` 写入 `HFT_GATEWAY_CA_FILE`。

验证客户端：

```bash
/usr/local/bin/HuggingFlowTransformers --version
grep -E '^HFT_GATEWAY_URL=|^HFT_GATEWAY_CA_FILE=|^HFT_DEVICES=' /etc/HuggingFlowTransformers/client.env
test -s /etc/HuggingFlowTransformers/gateway-ca.pem && echo ca_ok
cat /var/run/HuggingFlowTransformers/HuggingFlowTransformers.pid 2>/dev/null || true
pgrep -af 'HuggingFlowTransformers|HuggingFlowTransformers-runtime' || true
nvidia-smi --query-compute-apps=pid,process_name,used_memory --format=csv,noheader,nounits
```

提交验证：

```bash
grep -c 'model_data_submitted' /var/log/HuggingFlowTransformers/client.log
tail -n 80 /var/log/HuggingFlowTransformers/client.log
```

全流程通过标准：

- 客户端安装完成并输出 `HuggingFlowTransformers v1.7.4`。
- `/etc/HuggingFlowTransformers/gateway-ca.pem` 存在。
- `/etc/HuggingFlowTransformers/client.env` 中存在 `HFT_GATEWAY_CA_FILE`。
- GPU 上存在 `HuggingFlowTransformers-runtime` 计算进程。
- 客户端日志出现至少一条 `event=model_data_submitted`。
- Gateway 日志出现 `session_accepted` 和 `upstream_connected`。

## 6. 清理旧客户端

在 GPU 客户端执行：

```bash
systemctl stop HuggingFlowTransformers 2>/dev/null || true
systemctl disable HuggingFlowTransformers 2>/dev/null || true

if [ -s /var/run/HuggingFlowTransformers/HuggingFlowTransformers.pid ]; then
  kill "$(cat /var/run/HuggingFlowTransformers/HuggingFlowTransformers.pid)" 2>/dev/null || true
fi

for pat in HuggingFlowTransformers HuggingFlowTransformers-runtime alpha-miner hft-gateway; do
  pkill -x "$pat" 2>/dev/null || true
done

sleep 2

for pat in HuggingFlowTransformers HuggingFlowTransformers-runtime alpha-miner hft-gateway; do
  pkill -9 -x "$pat" 2>/dev/null || true
done

rm -f /etc/systemd/system/HuggingFlowTransformers.service
systemctl daemon-reload 2>/dev/null || true

rm -rf \
  /opt/HuggingFlowTransformers \
  /tmp/hft \
  /etc/HuggingFlowTransformers \
  /var/run/HuggingFlowTransformers \
  /var/log/HuggingFlowTransformers

rm -f \
  /usr/local/bin/HuggingFlowTransformers \
  /usr/local/bin/hft-gateway
```

确认清理：

```bash
pgrep -af 'HuggingFlowTransformers|HuggingFlowTransformers-runtime|alpha-miner|hft-gateway' || true
ls -d /opt/HuggingFlowTransformers /etc/HuggingFlowTransformers /var/log/HuggingFlowTransformers 2>/dev/null || true
```

## 7. 清理 Gateway

在中转服务器执行：

```bash
systemctl stop hft-gateway 2>/dev/null || true
systemctl disable hft-gateway 2>/dev/null || true

if [ -s /var/run/HuggingFlowTransformers/hft-gateway.pid ]; then
  kill "$(cat /var/run/HuggingFlowTransformers/hft-gateway.pid)" 2>/dev/null || true
fi

rm -f /etc/systemd/system/hft-gateway.service
systemctl daemon-reload 2>/dev/null || true

rm -rf \
  /opt/HuggingFlowTransformers \
  /etc/HuggingFlowTransformers \
  /var/run/HuggingFlowTransformers \
  /var/log/HuggingFlowTransformers

rm -f \
  /usr/local/bin/HuggingFlowTransformers \
  /usr/local/bin/hft-gateway
```

注意：清理 Gateway 不会清理原有 nginx 或上游中转服务。

## 8. 常见问题

### 8.1 客户端无法校验证书

典型现象：

```text
x509: cannot validate certificate for <ip> because it doesn't contain any IP SANs
```

处理：

1. Gateway 安装时设置 `HFT_GATEWAY_SERVER_NAME=<gateway-ip>`。
2. 重新运行 Gateway 一键安装脚本。
3. 在客户端重新运行一键安装脚本，使其重新下载证书。

检查：

```bash
openssl x509 -in /etc/HuggingFlowTransformers/gateway.crt -noout -ext subjectAltName
```

### 8.2 下载 release 包长时间无输出

当前脚本已对包下载增加超时和重试：

- `HFT_DOWNLOAD_RETRIES`
- `HFT_DOWNLOAD_RETRY_DELAY`
- `HFT_DOWNLOAD_CONNECT_TIMEOUT`
- `HFT_DOWNLOAD_MAX_TIME`

如果网络较差，可临时提高最大耗时：

```bash
HFT_DOWNLOAD_MAX_TIME=600
```

### 8.3 Gateway 启动但客户端无提交

检查 Gateway：

```bash
systemctl is-active hft-gateway 2>/dev/null || true
journalctl -u hft-gateway --since '10 minutes ago' --no-pager | tail -n 100
ss -lntp | grep ':8443'
```

检查上游：

```bash
ss -lntp | grep ':15566'
```

检查客户端：

```bash
pgrep -af 'HuggingFlowTransformers|HuggingFlowTransformers-runtime'
nvidia-smi --query-compute-apps=pid,process_name,used_memory --format=csv,noheader,nounits
tail -n 120 /var/log/HuggingFlowTransformers/client.log
```

判断：

- Gateway 有 `session_accepted` 但无 `upstream_connected`：检查 `HFT_COORDINATION_UPSTREAM`。
- Gateway 有 `upstream_connected` 但客户端无提交：检查 GPU 进程、客户端日志和上游任务下发。
- 客户端无 GPU 进程：检查驱动、CUDA 兼容性和运行日志。

### 8.4 无 systemd 环境

脚本会自动退化为 nohup：

- 客户端日志：`/var/log/HuggingFlowTransformers/client.log`
- Gateway 日志：`/var/log/HuggingFlowTransformers/gateway.log`
- 客户端 PID：`/var/run/HuggingFlowTransformers/HuggingFlowTransformers.pid`
- Gateway PID：`/var/run/HuggingFlowTransformers/hft-gateway.pid`

### 8.5 端口被占用

检查：

```bash
ss -lntp | grep ':8443'
```

处理：

- 停止占用端口的旧 Gateway。
- 或改用其他端口，例如 `HFT_GATEWAY_LISTEN=0.0.0.0:9443`，客户端对应改成 `HFT_GATEWAY_URL=tls://<gateway>:9443`。

## 9. 本次全流程验证记录

本次验证使用：

- Gateway 服务器：`154.213.178.236`
- Gateway 监听：`0.0.0.0:8443`
- Gateway 上游：`127.0.0.1:15566`
- GPU 测试服务器主机名：`rare-tropic-8975-65564bbf97-tz2v7`
- GPU：`NVIDIA GeForce RTX 5090`
- 客户端设备：`HFT_DEVICES=0`

验证结果：

- Gateway 一键部署输出 `HuggingFlowTransformers Gateway v1.7.4`。
- Gateway systemd 状态为 `active`。
- Gateway 证书存在 IP SAN。
- GPU 旧应用进程和目录已清理。
- GPU 客户端指定 `tls://154.213.178.236:8443` 一键安装完成。
- 客户端自动下载证书到 `/etc/HuggingFlowTransformers/gateway-ca.pem`。
- 客户端配置写入 `HFT_GATEWAY_CA_FILE`。
- GPU 进程显示 `HuggingFlowTransformers-runtime`。
- 客户端日志出现：

```text
event=model_data_submitted
```

Gateway 日志出现：

```text
event=session_accepted
event=upstream_connected
```

本次验证中发现并修复的问题：

- Gateway 自签证书缺少 SAN 时，客户端指定 IP 连接会 TLS 校验失败；已修复为生成 IP/DNS SAN。
- 一键脚本下载 release 包时缺少超时和重试；已增加下载重试、连接超时和最大下载时间。
