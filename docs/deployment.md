---
title: VPS 部署指南
description: m2h 作为长期文档服务部署到 Linux VPS 的完整方案：systemd、Nginx 反向代理、Tinyauth 认证与安全模型。
tags:
  - 部署
  - 安全
create_date: 2026-08-30
update_date: 2026-08-30
---

# VPS 部署指南

m2h 是面向**可信 Markdown 内容**的轻量只读 Web 文档服务：它负责安全地把显式指定的文档 root 暴露为 Web 内容；身份认证、TLS 和公网访问控制由反向代理负责。本文给出把 m2h 作为 systemd 常驻服务、经 Nginx 对外发布的推荐配置。

## 目标架构

```text
Internet
   │
   ▼
┌──────────────────────┐
│ Nginx                │
│ TLS / HTTP2          │
│ access log           │
│ HSTS                 │
└──────────┬───────────┘
           │
     optional auth_request
           │
┌──────────▼───────────┐
│ Tinyauth             │
└──────────┬───────────┘
           │
           ▼
    127.0.0.1:8793
┌──────────────────────┐
│ m2h                  │
│ HTTP hardening       │
│ CSP / security hdr   │
│ document boundary    │
│ asset boundary       │
│ /healthz             │
└──────────┬───────────┘
           │ read only
           ▼
       /srv/m2h/docs
```

## 安全模型

m2h 自身不负责身份认证，发布边界由以下规则构成：

1. CLI 指定的 root 是"允许发布的内容边界"。单文件模式下，为支持 Markdown 相对附件引用，文件所在目录是附件解析边界，因此同目录中的其他非隐藏、非主动类型附件也可能通过 `/assets/` 被访问。VPS 长期服务推荐使用专门的文档目录作为 root。
2. 目录 root 下任何包含 `.` 开头路径段的内容（`.env`、`.git/`、`foo/.private/`）不属于 Web 可见内容；显式以单文件路径启动（如 `m2h notes/.private.md`）是主动发布行为，不受此限制。发布策略同时作用于请求路径与符号链接解析后的 canonical 目标：可见别名无法指向隐藏文件或主动 Web 内容绕过边界。
3. Markdown 文档视为管理员提供的**可信内容**：raw HTML 按输入原样渲染。认证只控制谁可以读取文档，并不能把恶意 Markdown 转换为可信内容——不要使用 m2h 直接托管未经审核的用户上传 Markdown。如需展示不可信内容，应在反向代理层隔离 origin，或等待独立的 safe rendering mode。
4. HTML/JS/CSS 等主动 Web 文件不得通过 `/assets/` 运行（返回 404）；SVG 等普通附件允许访问，但 `/assets` 响应携带 `Content-Security-Policy: sandbox; default-src 'none'`，直接导航到 SVG 时其中的内嵌脚本不会执行。
5. 不要试图把秘密文件放进 root 再依赖文件名规则保护：真正的秘密必须位于 root 之外。
6. VPS 反向代理部署默认只监听 `127.0.0.1`。

## 安装与启动

从 [GitHub Releases](https://github.com/lz-wang/m2h/releases/latest) 下载对应平台的二进制放到 `/usr/local/bin/m2h`，或使用 Homebrew（Linux 同样支持 `brew install lz-wang/tap/m2h`）。

VPS 推荐（也是 systemd unit 使用的）启动命令：

```console
m2h /srv/m2h/docs \
  --host 127.0.0.1 \
  --port 8793 \
  --no-open
```

默认监听 `127.0.0.1:8793`。通过 Nginx/Caddy/Traefik 反向代理部署时保持 loopback 监听，仅由反向代理暴露公网入口；需要直接向局域网提供服务时才使用 `--host 0.0.0.0`。

## 专用用户与文件权限

为服务创建无 shell 的系统用户：

```console
useradd \
  --system \
  --no-create-home \
  --shell /usr/sbin/nologin \
  m2h
```

文档目录不必由 `m2h` 用户拥有。推荐由 deploy 用户或 git 同步流程维护 `/srv/m2h/docs`，`m2h` 只需要只读访问：

```text
deploy user / git sync
        │
        ▼
   /srv/m2h/docs
        │
      read-only
        ▼
       m2h
```

`m2h` 用户对文档目录只需 `r-x`（目录）与 `r--`（文件）。这样即使 HTTP 进程未来出现漏洞，也无法修改 Markdown 内容。

## systemd unit

`/etc/systemd/system/m2h.service`：

```ini
[Unit]
Description=m2h Markdown document service
After=network-online.target
Wants=network-online.target

[Service]
Type=simple

User=m2h
Group=m2h

ExecStart=/usr/local/bin/m2h \
    /srv/m2h/docs \
    --host 127.0.0.1 \
    --port 8793 \
    --no-open

Restart=on-failure
RestartSec=2s
TimeoutStopSec=15s

NoNewPrivileges=true
PrivateTmp=true
PrivateDevices=true

ProtectSystem=strict
ProtectHome=true
ProtectKernelTunables=true
ProtectKernelModules=true
ProtectControlGroups=true

LockPersonality=true
RestrictSUIDSGID=true

RestrictAddressFamilies=AF_UNIX AF_INET AF_INET6

CapabilityBoundingSet=
AmbientCapabilities=

ReadOnlyPaths=/srv/m2h/docs

UMask=0027

[Install]
WantedBy=multi-user.target
```

这里的要点是专用用户、loopback 监听、只读文档目录、`NoNewPrivileges` 与 `ProtectSystem`；没有使用 `SystemCallFilter=`、`DynamicUser=` 等更激进的选项，避免增加部署兼容风险。

启用并验证：

```console
systemctl daemon-reload
systemctl enable --now m2h
curl -fsS http://127.0.0.1:8793/healthz
```

## 健康检查

`GET /healthz` 返回 `200` 与 `ok`（`Cache-Control: no-store`），只回答 HTTP 进程是否已初始化并可接受请求：不读取、不扫描文档，`git pull` 瞬时的文件 rename 不会导致健康检查失败与误重启。`HEAD` 同样可用，其他方法返回 `405`。

本机监控（systemd、Uptime Kuma 等）直接探测 upstream：

```console
curl -fsS http://127.0.0.1:8793/healthz
```

**公网入口不应对 `/healthz` 建立绕过认证的公开 bypass**——公网探测仍经 Tinyauth；如确需公网 health probe，在 Nginx 层单独设计 ACL。m2h 自身不感知认证。

## Nginx 反向代理

对 `docs.example.com` 的推荐配置：

```nginx
server {
    listen 443 ssl;
    server_name docs.example.com;

    # TLS configuration ...

    add_header Strict-Transport-Security "max-age=31536000" always;

    location / {
        proxy_pass http://127.0.0.1:8793;

        proxy_http_version 1.1;

        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;

        proxy_read_timeout 60s;
        proxy_buffering on;
    }
}
```

- **必须部署在 origin root**（`https://docs.example.com/`），不要做 `example.com/m2h/` 子路径 rewrite：WebUI、API、runtime、document、asset 地址全部按 origin root 构建。
- 不需要 WebSocket / `Upgrade` / `Connection: upgrade` 配置，m2h 没有 WebSocket。
- HSTS 属于 TLS 终结层，由 Nginx 设置；m2h 看不到 TLS 连接，因此自身不发送该头。

## Tinyauth 认证（可选）

m2h 不需要知道 Tinyauth 的存在。Tinyauth 为 Nginx 提供专用 `/api/auth/nginx` endpoint，配合 `auth_request` 使用：

```nginx
location / {
    auth_request /tinyauth;

    auth_request_set $redirection_url
        $upstream_http_x_tinyauth_location;

    error_page 401 403 =302 $redirection_url;

    proxy_pass http://127.0.0.1:8793;

    proxy_set_header Host $host;
    proxy_set_header X-Real-IP $remote_addr;
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    proxy_set_header X-Forwarded-Proto $scheme;
}

location /tinyauth {
    internal;

    proxy_pass http://127.0.0.1:3000/api/auth/nginx;

    proxy_pass_request_body off;
    proxy_set_header Content-Length "";

    proxy_set_header x-forwarded-for $remote_addr;
    proxy_set_header x-real-ip $remote_addr;
    proxy_set_header x-forwarded-proto $scheme;
    proxy_set_header x-forwarded-host $host;
    proxy_set_header x-forwarded-uri $request_uri;
}
```

`auth_request` 必须位于 `location /`，这样自动保护全部路由（`/`、`/doc/`、`/raw/`、`/api/`、`/assets/`、`/runtime/`、`/ui/`）。只保护 `/doc/` 会让 `/raw/private.md`、`/api/document`、`/assets/private.pdf` 绕过认证。

如果 Tinyauth 需要正确识别客户端 IP（尤其使用 IP ACL 时，这是必需的），需将 Nginx 的实际来源地址/CIDR 加入 `TINYAUTH_AUTH_TRUSTEDPROXIES`，否则 Tinyauth 不应信任代理传来的真实客户端 IP。注意这是 `Nginx → Tinyauth` 的信任关系，与后文 m2h 自己不解析 `X-Forwarded-For` 的原则互不冲突。

## 日志职责

经 Nginx 后 m2h 的请求日志显示的客户端 IP 基本是 `127.0.0.1`，这是预期行为：

```text
Nginx access.log → 公网 client IP / request audit
m2h log          → application request / error / latency
```

m2h 不解析 `X-Forwarded-For`，也不引入 trusted proxy 概念——无条件信任 XFF 会伪造客户端 IP。如果未来确实需要在 m2h 日志中看到真实 IP，再单独评估 trusted proxy CIDR 方案。

## 能力边界

| 能力 | 归属 |
| --- | --- |
| Markdown render | m2h |
| 文件访问 sandbox | m2h |
| asset policy | m2h |
| CSP | m2h |
| health check | m2h |
| HTTP lifecycle | m2h |
| TLS | Nginx |
| HSTS | Nginx |
| 用户登录 | Tinyauth |
| OAuth | Tinyauth |
| IP access log | Nginx |
| process restart | systemd |
| 文档更新 | Git/rsync |
| 文件写入 | 不支持 |
