<p align="center">
  <img src="web/public/favicon.svg" alt="m2h Logo" width="120" height="120">
</p>

# m2h

`m2h` 是一个 Markdown → HTML 命令行工具，支持将 Markdown 转换为可离线打开的 HTML，以及在浏览器中实时预览单个文件或目录。

## 安装

### Homebrew

macOS 和 Linux：

```console
$ brew install lz-wang/tap/m2h
```

升级已安装的版本：

```console
$ brew upgrade m2h
```

### GitHub Releases

从 [GitHub Releases](https://github.com/lz-wang/m2h/releases/latest) 下载对应平台和架构的压缩包，解压后将 `m2h`（Windows 为 `m2h.exe`）加入 `PATH`。

```text
m2h_<version>_linux_{amd64,arm64}.tar.gz
m2h_<version>_darwin_{amd64,arm64}.tar.gz
m2h_<version>_windows_{amd64,arm64}.zip
```

每个发布包均提供同名 `.sha256` 校验文件。

## 快速开始

```console
$ m2h README.md
$ m2h web docs
```

使用 `m2h --version` 查看当前版本。

## 转换为 HTML

```console
# 转换单个文件，生成 README.html
$ m2h README.md

# 指定输出文件
$ m2h README.md --output public/index.html

# 转换目录中的 Markdown，并保留目录结构
$ m2h docs --output public/docs --depth 3 --glob '**/plan_*.md'
```

目录转换默认会复制非 Markdown 资源；使用 `--copy-assets=false` 可关闭。相对 Markdown 链接会改为对应的 `.html` 链接，图片和其他本地资源路径保持不变。

转换成功后，m2h 会向标准输出打印转换数量、复制资源数量（如有）与每个生成 HTML 的绝对路径：

```text
Converted 1 Markdown file.
Output HTML files:
- /work/project/README.html
```

| 选项 | 说明 |
| --- | --- |
| `--output`, `-o` | 单文件的目标 HTML，或目录转换的目标目录。 |
| `--glob` | 按相对输入目录的路径筛选 Markdown，例如 `'**/guide_*.md'`。仅目录可用。 |
| `--depth`, `-d` | 最大递归深度；默认 `4`。 |
| `--copy-assets` | 是否复制非 Markdown 资源；默认 `true`。 |
| `--mode` | 页面主题：`light`、`dark` 或 `auto`；默认 `auto`。 |
| `--width` | 正文宽度：`standard`（980px）、`wide`（1280px）或 `full`；默认 `standard`。 |

## 在浏览器中预览

```console
# 预览单个文件；保存后局部刷新正文，不显示文件侧边栏
$ m2h web README.md

# 预览目录；提供文件树、搜索、主题、正文宽度与文档目录
$ m2h web docs --mode dark --width wide
```

服务默认监听 `http://127.0.0.1:8793` 并自动打开浏览器，按 `Ctrl+C` 停止服务；加 `--no-open` 可仅启动服务。

| 选项 | 说明 |
| --- | --- |
| `--host` | 监听地址；默认 `127.0.0.1`。 |
| `--port`, `-p` | 监听端口；默认 `8793`。 |
| `--open` / `--no-open` | 启动后是否打开系统默认浏览器；默认打开，使用 `--no-open` 关闭。 |
| `--mode` | 页面主题：`light`、`dark` 或 `auto`；默认 `auto`。 |
| `--width` | 正文宽度：`standard`、`wide` 或 `full`；默认 `standard`。 |
| `--toc` | 是否显示文档目录；默认 `true`，关闭使用 `--toc=false`。 |
| `--glob` | 按相对输入目录的路径筛选 Markdown。仅目录可用。 |
| `--depth`, `-d` | 最大递归深度；默认 `4`。仅目录可用。 |

单文件与目录预览共用同一 Web 界面：保存 Markdown 后通过局部刷新更新当前文档正文，保留主题、宽度、文档目录与导航状态。预览选项始终保留在 URL 中，非默认主题、正文宽度与关闭的文档目录（`?toc=false`）也会保留，自动主题、标准宽度与开启的文档目录则省略，便于分享和返回。目录预览额外提供可搜索的文件侧边栏；侧边栏底部可在新页面打开 GitHub 仓库与当前版本发布信息，开发版进入 Releases 列表；正文右侧的文档目录列出 H2–H4 标题，滚动时高亮当前小节，点击可跳转。

## GFM 兼容性

默认转换与 `web` 共用同一套 Markdown 解析和 HTML 渲染规则，因此 GFM、脚注、GitHub Alerts、数学公式、Mermaid、原始 HTML 等语义在两种模式下保持一致。

| 特性 | 简单示例 |
| --- | --- |
| 标题与锚点 | `## 安装`；可链接到 `#安装`。重复标题会自动区分。 |
| 强调与删除线 | `**加粗**`、`*斜体*`、`~~删除~~` |
| 列表与任务列表 | `- 项目`、`1. 第一步`、`- [x] 已完成` |
| 引用与分割线 | `> 引用内容`、`---` |
| 链接与图片 | `[网站](https://example.com)`、`![图片](images/demo.png)` |
| 自动链接 | `<https://example.com>` 或 `https://example.com` |
| 行内与围栏代码 | `` `go test` ``；围栏以 ```` ```go ```` 开始、以 ```` ``` ```` 结束。 |
| 表格 | 两列的表格，例如“名称”和“值”。 |
| 脚注 | `说明[^1]`，并在文末写 `[^1]: 补充说明` |
| emoji 短代码 | `:rocket:` → 🚀 |
| GitHub Alerts | `> [!NOTE]` 后接提示内容。 |
| 数学公式 | `$E = mc^2$`；行间公式使用 `$$...$$` |
| Mermaid 图表 | 使用 ```` ```mermaid ```` 围栏，例如 `flowchart LR` 与 `A --> B`。 |
| 原始 HTML | 直接渲染，例如 `<details>内容</details>`；`web` 会通过安全附件路由加载标签中的相对本地资源。 |

数学公式和 Mermaid 图表的运行时资源已包含在输出中，生成的 HTML 可离线打开。

## 许可证

[MIT](LICENSE)
