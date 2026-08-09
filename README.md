# m2h

[![codecov](https://codecov.io/gh/lz-wang/m2h/graph/badge.svg?token=iNo6LuOlzm)](https://codecov.io/gh/lz-wang/m2h)

`m2h` 是一个使用 Go 实现的命令行工具，用于把 Markdown 转换为 HTML，或在浏览器、终端中预览 Markdown。

> 当前状态：CLI 骨架和版本命令已经可用；`convert`、`preview`、`view` 仍处于后续阶段，调用时会返回明确的未实现错误。具体进度见 [PROGRESS.md](PROGRESS.md)。

## 命令概览

```text
m2h version

m2h convert <file|directory>
    --output/-o
    --glob
    --depth/-d
    --mode
    --copy-assets
    --unsafe-html

m2h preview <file|directory>
    --host
    --port/-p
    --browser
    --mode
    --unsafe-html
    --glob
    --depth/-d

m2h view <file>
    --mode
```

## 查看版本

```console
$ m2h version
dev-20260807-a12bc34

$ m2h --version
dev-20260807-a12bc34
```

开发构建使用 `dev-<commit date>-<commit7>`，例如 `dev-20260807-a12bc34`；正式发布使用语义化版本，例如 `1.2.3`。开发版本日期取 Git commit date，不取构建日期。

## 转换为 HTML

转换单个文件：

```console
$ m2h convert README.md
# 写入 README.html

$ m2h convert README.md --output public/index.html
```

批量转换目录：

```console
$ m2h convert docs --output public/docs
$ m2h convert docs/ --output public/docs --depth 3 --glob '**/plan_*.md'
```

目录末尾是否带 `/` 不影响语义。批量模式保留 Markdown 的相对目录结构；未指定 `--output` 时，HTML 写在源 Markdown 旁边。`--copy-assets` 默认为 `true`，非 Markdown 文件按原相对路径复制。

主要选项：

- `--output, -o`：单文件的目标 HTML，或批量模式的目标目录。
- `--depth, -d`：目录递归深度，默认 `2`（输入目录和向下两层）。
- `--glob`：按相对输入根目录、使用 `/` 分隔的路径匹配 Markdown。
- `--mode`：`light`、`dark` 或 `auto`，默认 `auto`。
- `--unsafe-html`：显式允许 Markdown 中的原始 HTML，默认关闭。

本地资源路径保持不变，本地 Markdown 链接会改写为 HTML 链接：

```text
images/demo.png  -> images/demo.png
guide.md         -> guide.html
../guide.md      -> ../guide.html
https://...      -> 不修改
#install         -> 不修改
```

## 在浏览器中预览

预览单个 Markdown 文件：

```console
$ m2h preview README.md
$ m2h preview README.md --host 127.0.0.1 --port 8793 --browser
```

单文件模式只显示 GitHub 风格正文，并监听文件所在目录以兼容编辑器的 atomic save；内容变化后通过 SSE 刷新页面。

浏览一个 Markdown 目录：

```console
$ m2h preview docs
$ m2h preview docs --glob '**/*.md' --depth 4 --mode dark
```

目录模式提供文件树、当前文档标题和手动刷新按钮。文档路由形如 `/doc/design/architecture.md`，直接刷新仍停留在当前文档。首次打开依次选择 `README.md`、`index.md`、按相对路径排序的第一个 Markdown；没有 Markdown 时显示空状态。

服务默认监听 `127.0.0.1:8793`，`--browser` 默认关闭，`--mode` 默认 `auto`。目录模式不自动监听文件；点击刷新后，下一次打开文档会从磁盘读取最新内容。

## 在终端中预览

```console
$ m2h view README.md
$ m2h view README.md --mode dark
```

`view` 使用终端样式渲染单个 Markdown 文件，不启动 Web 服务。

## Markdown 与页面样式

- Markdown 语法固定为标准 GFM，不提供额外扩展选项；代码块支持语法高亮。
- HTML 使用 `github-markdown-css`，正文最大宽度为 `980px`。
- 桌面端正文 padding 为 `45px`；宽度不超过 `767px` 时为 `15px`。
- raw HTML 与危险 URL 默认不渲染，只有显式传入 `--unsafe-html` 才允许 raw HTML。

## 参数错误

未知参数和不适用于当前输入类型的参数会直接失败，不会静默忽略：

```text
Error: unknown option
Error: --glob can only be used when serving a directory
```

## 从源码构建

开发环境入口统一由 Makefile 提供：

```console
$ make setup
$ make build
$ make test
$ make check
$ make help
```

## 许可证

[MIT](LICENSE)
