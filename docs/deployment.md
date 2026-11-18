---
title: VPS 部署指南
description: m2h 作为长期文档服务部署到 Linux VPS 的完整方案：systemd、Nginx 反向代理、Tinyauth 认证与安全模型。
tags:
  - 部署
  - 安全
create_date: 2026-08-30
update_date: 2026-11-18
---

# VPS 部署指南

m2h 是面向**可信 Markdown 内容**的轻量只读 Web 文档服务：它负责安全地把显式指定的文档 root 暴露为 Web 内容；TLS 和公网访问控制由反向代理负责，身份认证既可交给反向代理，也可启用 m2h 内置认证（见[内置认证](#内置认证可选-v0270-起)一节）。本文给出把 m2h 作为 systemd 常驻服务、经 Nginx 对外发布的推荐配置。

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

WebUI 为普通 SVG 的矢量显示读取图片源码，`connect-src` 允许同源、HTTP(S)、data
与 blob 图片来源。远程 SVG 仍受浏览器 CORS 限制；源码清理后在独立样式作用域中
显示，不执行脚本、事件处理器或链接导航。反向代理叠加 CSP 时需允许所用图片来源。

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

### 可选 CDN 加载

当 VPS 出口带宽或延迟影响图表加载时，在启动参数中加入 `--cdn`：

```console
m2h /srv/m2h/docs --host 127.0.0.1 --port 8793 --no-open --cdn
```

浏览器会按需直接从 `https://cdn.jsdelivr.net` 加载固定版本的 Mermaid、
KaTeX（含样式与字体）和表格排序依赖；主程序、文档、API 与附件仍由
m2h 服务。下面的 systemd unit 若需启用，只需在 `ExecStart` 中追加 `--cdn`，
执行 `systemctl daemon-reload` 并重启服务，刷新页面即可生效，文档深链接也使用同一配置。

默认关闭，`--no-cdn` 显式使用内嵌资源。CDN 模式依赖访问者的网络，实际速度应以
部署后的访问体验为准；CDN 无法访问时可移除 `--cdn` 或使用 `--no-cdn` 后重启服务。
m2h 会在 CSP 的脚本、样式与字体来源中加入 `https://cdn.jsdelivr.net`，仍禁止
内联脚本与 `eval`；若反向代理额外设置了 CSP，也需允许这些来源，避免浏览器拦截。
附件的独立 sandbox 策略继续生效。此选项只控制 Web 文档服务。

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

- 默认部署在 origin root（`https://docs.example.com/`）；部署到 URL 子路径时使用 `--base-path`，见下节。
- 不需要 WebSocket / `Upgrade` / `Connection: upgrade` 配置，m2h 没有 WebSocket。
- HSTS 属于 TLS 终结层，由 Nginx 设置；m2h 看不到 TLS 连接，因此自身不发送该头。

## 子路径部署

同一个二进制可通过 `--base-path` 部署到 `https://example.com/docs/`，无需重新构建 WebUI：

```console
m2h /srv/m2h/docs --host 127.0.0.1 --port 8793 --base-path /docs --no-open
```

若使用前面的 systemd unit，在 `ExecStart` 中加入 `--base-path /docs`，重新加载 unit
并重启服务。启动日志、自动打开浏览器及文档链接都会包含该前缀；UI、API、附件、
原始 Markdown、富内容运行时与健康检查也统一挂载到 `/docs/` 下。

`--base-path` 默认 `/`，支持 `/docs`、`/docs/` 和 `/apps/docs`，末尾 `/` 会被规范化。
路径段只接受 ASCII 字母、数字及 `-`、`_`、`.`、`~`，不接受空路径段、独立的 `.` 或
`..`、百分号编码、空格、查询串与片段。该选项只适用于浏览服务，`export` 与 `check`
不接受。前缀不参与文件系统或多 root 的路径解析。

Nginx 保留请求前缀，由 m2h 自行处理：

```nginx
server {
    listen 443 ssl;
    server_name example.com;

    # TLS configuration ...
    add_header Strict-Transport-Security "max-age=31536000" always;

    location = /docs {
        return 308 /docs/$is_args$args;
    }

    location ^~ /docs/ {
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

`proxy_pass` **不要添加末尾 `/`**：不带 URI 的配置会保留原始请求路径，
`/docs/api/files` 仍以相同路径交给 m2h，参见 [Nginx proxy_pass 文档](https://nginx.org/en/docs/http/ngx_http_proxy_module.html#proxy_pass)。
无需 `rewrite`、`sub_filter` 或 HTML `<base>`；m2h 不从 `X-Forwarded-Prefix` 等代理头
推断路径。一个实例只使用一个显式前缀；前缀外的路由返回 404，`/docs` 自动以 308
重定向到 `/docs/` 并保留查询参数。

启用 Tinyauth 时，将下节的 `auth_request` 及对应错误跳转配置放在
`location ^~ /docs/` 内，统一保护 UI、API、原始 Markdown、附件与运行时资源。
不要为 `/docs/raw/`、`/docs/assets/` 或 `/docs/healthz` 添加绕过认证的公网入口。

启动后验证本机 upstream 和代理入口（文档路径替换成实际可见文档）：

```console
curl -I http://127.0.0.1:8793/docs/
curl -fsS http://127.0.0.1:8793/docs/healthz
curl -fsS http://127.0.0.1:8793/docs/api/files
curl -I https://example.com/docs/
curl -I https://example.com/docs/doc/README.md
```

本机健康监控须改用 `/docs/healthz`；公网检查仍遵循认证策略。浏览器验收应同时检查
深链接刷新、搜索跳转、浏览器前进后退、图表、图片及复制链接均保留 `/docs` 前缀。
`--cdn` 与子路径部署可同时使用；外部 CDN URL 不添加前缀。

### 独立代理 Smoke

仓库维护者可在安装 Nginx 与 Python 3 后验证实际代理行为：

```console
make build
python3 scripts/nginx-smoke.py ./m2h
```

脚本使用临时文档、临时 Nginx 配置和本机动态端口，验证 `/docs`、`/apps/docs`
下的 API、HEAD、重定向及含中文、空格和百分号的附件，结束后清理子进程和临时文件。
它不修改已有 Nginx 服务配置；CI 在浏览器回归后单独执行该 Smoke。
测试不包含公网 TLS 或 Tinyauth 登录验收。

## 内置认证（可选，v0.27.0 起）

m2h 自带可选的认证与协作层。默认启动（不带 `--auth`）保持零状态：不建数据库、不读认证配置、不注册登录接口。显式选择认证模式后，全部文档、附件与搜索接口都要求登录，并解锁文档标注与 API Token。

### 启动

```console
# 本地账号（明文或 Argon2id PHC 哈希）
M2H_LOCAL_USERS_JSON='{"alice":"password-1"}' \
  m2h docs --auth local --host 127.0.0.1 /srv/docs

# LDAP（LDAPS 或 StartTLS + 用户白名单）
M2H_LDAP_URL=ldaps://ldap.example.internal:636 \
M2H_LDAP_BIND_DN='uid=service,ou=people,dc=example,dc=internal' \
M2H_LDAP_BIND_PASSWORD=... \
M2H_LDAP_USER_BASE_DN='ou=people,dc=example,dc=internal' \
M2H_LDAP_USER_FILTER='(uid=%s)' \
M2H_LDAP_ALLOWED_USERS='alice,bob' \
  m2h docs --auth ldap /srv/docs
```

### 配置一览

| 变量 / 参数 | 作用 |
| --- | --- |
| `--auth <none\|local\|ldap>` | 认证模式；默认 `none`。仅设置环境变量不会启用认证 |
| `--data-dir` / `M2H_DATA_DIR` | SQLite 数据目录（默认为各操作系统用户数据目录下的 `m2h`）；不得位于任何发布的文档目录内 |
| `M2H_LOCAL_USERS_JSON` | 本地账号表，JSON 对象；值支持 `$argon2id$…` PHC 哈希 |
| `M2H_LDAP_*` | LDAP 连接、服务账号、检索基准/过滤器与白名单；过滤器中的 `%s` 会经 `ldap.EscapeFilter` 转义 |
| `M2H_PUBLIC_URL` | 对外完整 URL（如 `https://docs.example.com/docs`）；为 https 时会话 Cookie 附加 `Secure`。反向代理部署须设置此项，状态变更请求的 Origin 必须匹配其协议、主机和端口。m2h 不信任 `X-Forwarded-Proto` |

### 会话与数据

- 浏览器使用 `m2h_session` Cookie（HttpOnly、SameSite=Lax、路径跟随 `--base-path`），12 小时闲置 / 7 天绝对过期；登录失败统一报错并按用户名限流（15 分钟 10 次失败锁定该用户名）。
- 标注、会话与 Token 全部存在数据目录的 `m2h.db`（纯 Go SQLite 驱动，权限 0600）。数据库运行在 WAL 模式，**运行期间直接复制该文件不是一致性备份**——尚未合并进主文件的写入可能丢失。停机时复制文件即可；运行中备份使用 `sqlite3 <data-dir>/m2h.db "VACUUM INTO '<backup-path>/m2h.db'"`（或 SQLite Online Backup，`sqlite3 … ".backup '<backup-path>/m2h.db'"`）生成一致性快照，恢复时把快照放回数据目录重启即可。Schema 由版本号管理，旧库自动迁移，比程序新的库拒绝打开。
- 切换 Local/LDAP 后，旧模式签发的会话与 Token 失效。从 `M2H_LOCAL_USERS_JSON` 删除账号或从 `M2H_LDAP_ALLOWED_USERS` 移除用户后，重启服务以加载新配置。该账号的会话与 Token 在下一次请求时失效。
- LDAP 已有凭证只检查本地白名单，不检查目录中的禁用状态。禁用目录账号时，也须从白名单移除该用户并重启服务。同名账号重新加入配置后，未撤销且未过期的旧 Token 可能再次生效。移除账号前，应先在设置中撤销其所有 Token；不要依赖同名账号重建来撤销凭证。
- 登录验证最多同时执行 4 次，整个服务每分钟最多接受 60 次验证；超出时返回 429。反向代理还应限制登录请求速率，以减少网络和服务负载。
- 不要同时启用反向代理层的外置登录（如 Tinyauth）与 m2h 内置认证——两道登录会互相干扰；二选一。

### Agent 与 API Token

认证模式下可在 WebUI「API 令牌」设置中创建长期只读 Token，或在标注面板一键生成绑定当前文档版本的临时分享。Agent 通过 `Authorization: Bearer m2h_…` 搜索与读取原文，规范见 [Agent 知识库接入](agent-access.md)。

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

`auth_request` 必须位于覆盖整个服务的 `location /`（子路径部署为 `location ^~ /docs/`），这样自动保护全部路由（`/`、`/doc/`、`/raw/`、`/api/`、`/assets/`、`/runtime/`、`/ui/`）。只保护 `/doc/` 会让 `/raw/private.md`、`/api/document`、`/assets/private.pdf` 绕过认证。

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
