---
title: 浏览与导出 Markdown
description: 浏览选项、局域网访问与 HTML 导出用法。
tags:
  - 使用指南
create_date: 2026-11-18
update_date: 2026-11-20
---

# 浏览与导出 Markdown

## 打开文档

```console
m2h README.md
m2h docs
m2h docs wiki notes.md
m2h docs,wiki
```

默认监听 `127.0.0.1:8793` 并自动打开浏览器。打开目录或多根工作区时显示文档首页。单文件模式直接打开文档。
文档地址为 `/doc/<路径>`，打开该地址可直接阅读文档。部署在 `--base-path /docs` 时，首页地址为 `/docs/`。

选择左侧栏筛选框下方的「文档首页」图标可返回首页。滚动文件树时，图标固定在侧栏顶部。首页的「最新创建」和「最近更新」各显示最多 10 篇文档。选择任意一行可打开对应文档。

首页优先使用 [Frontmatter 日期字段](demos/frontmatter/00-index.md)。缺少有效日期时使用文件修改日期，创建日期会标为「估算」。单独的 `date` 字段不参与排序。

修改文件后重新打开文档即可读取最新内容。刷新首页时，浏览器会重新请求文件树和首页数据。

服务端首页元数据缓存的有效期为 30 秒。缓存有效时，服务端复用元数据，并校验返回的文档仍可读取。缓存过期后的下一次首页数据请求会触发重新扫描。

## 浏览选项

| 选项 | 说明 |
| --- | --- |
| `--host` | 监听地址 |
| `--base-path` | HTTP 部署路径，默认 `/`；支持 `/docs`、`/apps/docs`，详见 [子路径部署](deployment.md#子路径部署) |
| `--port`, `-p` | 监听端口，默认 `8793` |
| `--mode` | `light`、`dark` 或 `auto` |
| `--width` | `standard`、`wide` 或 `full` |
| `--glob` | Markdown 文件过滤规则 |
| `--depth`, `-d` | 目录最大递归深度，默认 `4` |
| `--open` / `--no-open` | 是否自动打开浏览器 |
| `--cdn` / `--no-cdn` | 是否从 jsDelivr CDN 按需加载 WebUI 富内容依赖，默认关闭 |
| `--toc` / `--no-toc` | 是否显示文档目录，默认开启（`true`）；只接受开关形式，不接受 `=值` |
| `--gitignore` / `--no-gitignore` | 是否遵循目录内的 `.gitignore` 规则，默认开启（`true`） |
| `--hidden` / `--no-hidden` | 是否包含隐藏文件和目录，默认关闭（`false`）；`.git`、`.ssh`、`.env` 等保护路径始终排除 |

更多选项：

```console
m2h --help
```

## 局域网访问

```console
m2h docs --host 0.0.0.0 --no-open
```

通过反向代理发布时，参见 [VPS 部署指南](deployment.md)。
富内容依赖默认使用内嵌资源，需要从 CDN 加载时，参见 [可选 CDN 加载](deployment.md#可选-cdn-加载)。

## 导出 HTML

```console
m2h export README.md
```

默认在 Markdown 文件旁生成同名 `.html`：

```text
README.md
README.html
```

指定输出文件名：

```console
m2h export README.md -o index.html
```

覆盖已有文件：

```console
m2h export README.md --force
```

导出的 HTML 内联 Markdown 页面样式；数学公式、Mermaid 和表格排序等
增强功能按需加载网络资源；只有包含可渲染 Mermaid 代码块时才加载 Mermaid。

查看全部选项：

```console
m2h export --help
```
