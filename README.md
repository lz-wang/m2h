# m2h

`m2h` 是一个 Markdown 命令行工具：将文件转换为可离线打开的 HTML，在浏览器中预览文件或目录，或直接在终端阅读。

## 安装

从 [GitHub Releases](https://github.com/lz-wang/m2h/releases/latest) 下载对应平台和架构的压缩包，解压后将 `m2h`（Windows 为 `m2h.exe`）加入 `PATH`。

```text
m2h_<version>_linux_{amd64,arm64}.tar.gz
m2h_<version>_darwin_{amd64,arm64}.tar.gz
m2h_<version>_windows_{amd64,arm64}.zip
```

每个发布包均提供同名 `.sha256` 校验文件。

## 快速开始

```console
$ m2h convert README.md
$ m2h preview docs --browser
$ m2h view README.md
```

使用 `m2h version` 或 `m2h --version` 查看当前版本。

## 转换为 HTML

```console
# 转换单个文件，生成 README.html
$ m2h convert README.md

# 指定输出文件
$ m2h convert README.md --output public/index.html

# 转换目录中的 Markdown，并保留目录结构
$ m2h convert docs --output public/docs --depth 3 --glob '**/plan_*.md'
```

目录转换默认会复制非 Markdown 资源；使用 `--copy-assets=false` 可关闭。相对 Markdown 链接会改为对应的 `.html` 链接，图片和其他本地资源路径保持不变。

| 选项 | 说明 |
| --- | --- |
| `--output`, `-o` | 单文件的目标 HTML，或目录转换的目标目录。 |
| `--glob` | 按相对输入目录的路径筛选 Markdown，例如 `'**/guide_*.md'`。仅目录可用。 |
| `--depth`, `-d` | 最大递归深度；默认 `2`。 |
| `--copy-assets` | 是否复制非 Markdown 资源；默认 `true`。 |
| `--mode` | 页面主题：`light`、`dark` 或 `auto`；默认 `auto`。 |
| `--width` | 正文宽度：`standard`（980px）、`wide`（1280px）或 `full`；默认 `standard`。 |
| `--unsafe-html` | 允许 Markdown 中的原始 HTML；默认关闭。 |

## 在浏览器中预览

```console
# 预览单个文件；保存文件后页面会自动刷新
$ m2h preview README.md --browser

# 预览目录；提供文件树、搜索、主题和正文宽度切换
$ m2h preview docs --browser --mode dark --width wide
```

服务默认监听 `http://127.0.0.1:8793`。按 `Ctrl+C` 停止服务。

| 选项 | 说明 |
| --- | --- |
| `--host` | 监听地址；默认 `127.0.0.1`。 |
| `--port`, `-p` | 监听端口；默认 `8793`。 |
| `--browser` | 服务启动成功后打开系统默认浏览器。 |
| `--mode` | 页面主题：`light`、`dark` 或 `auto`；默认 `auto`。 |
| `--width` | 正文宽度：`standard`、`wide` 或 `full`；默认 `standard`。 |
| `--unsafe-html` | 允许 Markdown 中的原始 HTML；默认关闭。 |
| `--glob` | 按相对输入目录的路径筛选 Markdown。仅目录可用。 |
| `--depth`, `-d` | 最大递归深度；默认 `2`。仅目录可用。 |

单文件预览会在文件变化后自动刷新。目录预览可通过界面中的刷新按钮重新扫描文件；选择、主题和正文宽度会保留在 URL 中，便于分享和返回。

## 在终端中阅读

```console
$ m2h view README.md
$ NO_COLOR=1 m2h view guide.md --mode dark
```

`view` 只接受单个 Markdown 文件，不启动 Web 服务。`--mode` 可选 `light`、`dark` 或 `auto`；`auto` 会尝试匹配终端背景。设置非空的 `NO_COLOR` 或将输出重定向时，会自动移除 ANSI 颜色。

## GFM 兼容性

`convert` 与浏览器 `preview` 使用相同的渲染规则。下表中的特性均可用于生成的 HTML 和浏览器预览；`view` 支持基础 GFM 与 emoji，脚注和 alerts 会分别以字面文本和普通引用显示，数学公式与 Mermaid 图表不在终端渲染。

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
| 原始 HTML | 默认不渲染；使用 `--unsafe-html` 后可写 `<details>内容</details>`。 |

数学公式和 Mermaid 图表的运行时资源已包含在输出中，生成的 HTML 可离线打开。

## 许可证

[MIT](LICENSE)
