<p align="center">
  <img src="web/public/favicon.svg" alt="m2h Logo" width="120" height="120">
</p>

# m2h

`m2h` 是一个轻量、零配置的 Markdown Web 文档浏览与 HTML 导出工具。

## 特性

- 直接浏览 Markdown 文件、目录或多个文档目录
- 文件树、搜索、文档目录、主题和正文宽度切换
- 支持分享文档链接、Markdown 链接和 Markdown 全文
- 支持 GFM、语法高亮、数学公式、Mermaid、脚注、Emoji 和 GitHub Alerts
- 支持 [Frontmatter 标题与日期元数据](docs/demos/frontmatter/00-index.md)、可排序表格、代码行号与长代码块折叠
- 图片与 Mermaid 图表支持 Lightbox 查看，可切换、通过工具栏或鼠标滚轮平滑缩放、拖动和旋转
- 文件修改后重新打开即可读取最新内容，刷新页面可重新扫描目录
- 输入 root 即发布边界：目录服务隐藏点开头路径，附件路由拒绝 HTML/JS/CSS 等主动 Web 内容，响应携带统一浏览器安全头
- 可将单个 Markdown 文件导出为 HTML
- 可检查 Markdown 文档的 Frontmatter、本地引用、锚点与结构问题（[25 条规则与演示](docs/demos/checkers/00-index.md)）

## 安装

### Homebrew

macOS 和 Linux：

```console
brew install lz-wang/tap/m2h
```

升级：

```console
brew upgrade m2h
```

### GitHub Releases

也可以从 [GitHub Releases](https://github.com/lz-wang/m2h/releases/latest)
下载对应平台的预编译版本。

支持：

* Linux amd64 / arm64
* macOS amd64 / arm64
* Windows amd64 / arm64

## 使用

### 浏览 Markdown

打开单个文件：

```console
m2h README.md
```

打开目录：

```console
m2h docs
```

同时打开多个目录或文件：

```console
m2h docs wiki notes.md
```

也支持逗号分隔：

```console
m2h docs,wiki
```

默认监听 `127.0.0.1:8793` 并自动打开浏览器。

直接向局域网提供文档服务：

```console
m2h docs --host 0.0.0.0 --no-open
```

反向代理部署时建议保持 loopback 监听，仅由代理暴露公网入口：

```console
m2h /srv/docs --host 127.0.0.1 --port 8793 --no-open
```

### VPS 部署

通过 Nginx 等反向代理长期运行（systemd、TLS、Tinyauth 认证与健康检查）时，参见 [VPS 部署指南](docs/deployment.md)。

常用选项：

| 选项 | 说明 |
| --- | --- |
| `--host` | 监听地址 |
| `--port`, `-p` | 监听端口，默认 `8793` |
| `--open` / `--no-open` | 是否自动打开浏览器 |
| `--mode` | `light`、`dark` 或 `auto` |
| `--width` | `standard`、`wide` 或 `full` |
| `--toc` | 是否显示文档目录 |
| `--glob` | Markdown 文件过滤规则 |
| `--depth`, `-d` | 目录最大递归深度，默认 `4` |

更多选项：

```console
m2h --help
```

### 导出 HTML

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
增强功能按需加载网络资源。

查看全部选项：

```console
m2h export --help
```

### 检查文档

`m2h check` 检查单个文件或目录的 Frontmatter、本地引用、锚点与文档结构：

```console
m2h check README.md
m2h check docs
```

输出遵循 `path:line:column` 约定，便于终端与 IDE 定位；发现 error（或
`--strict` 下存在 warning）时退出码为 `1`。交互式终端中只为问题等级和
总结结果着色：error 为红色、warning 为黄色、全部通过为绿色；重定向、管道、
`NO_COLOR` 环境变量及 `--format json` 保持无 ANSI 颜色。文本报告以统计摘要
作为最后一行，不再重复输出 `Error: check found ...`：

```text
docs/guide.md:42:17: error [local-target.missing]: target "images/topology.png" does not exist
docs/index.md:18:5: error [anchor.missing]: heading "#installation" does not exist in "guide.md"
```

`--depth` 与 `--glob` 和浏览命令使用同一文档范围；`--format json` 输出
结构化结果，`--strict` 把 warning 也视为失败。`--enable` 在默认规则之上
追加规则，`--disable` 移除规则且优先级更高，`all` 代表全部规则：

```console
m2h check docs --depth 8 --glob '**/*.md'
m2h check docs --format json
m2h check docs --strict
m2h check docs --enable all --disable image.alt-empty
```

完整的 25 条规则、默认开关、触发样例、行为边界与逐字输出见
[检查规则演示索引](docs/demos/checkers/00-index.md)；全部命令选项见
`m2h check --help`。

本地引用使用与 WebUI 相同的路径语义：`images/logo.png` 相对当前文档，
`/images/logo.png` 相对当前输入 root（不是宿主机文件系统根目录）；多 root 模式
始终锚定引用所在的 root。`//cdn.example.com/logo.png` 仍是协议相对网络 URL，
与带 scheme 的外链一样不会映射到本地文件。已经识别为本地、但解析后越出
当前 root 的引用会在 WebUI 中改写到专用的 404 地址，不再把原始相对 URL
交给浏览器重新解析；`m2h check` 同时报告 `local-target.outside-root`。

## Markdown 支持

m2h 支持常用 GFM Markdown，并提供以下扩展：

| 类别 | 支持内容与演示 |
| --- | --- |
| GitHub 风格 | 语法高亮、标题锚点、脚注、Emoji、GitHub Alerts 与可排序表格 |
| 富内容 | 数学公式（`$...$` / `$$...$$`，金额保持普通文本）、Mermaid 与 ZenUML |
| 元数据 | YAML Frontmatter；标题、日期别名和回退规则见 [Frontmatter 演示](docs/demos/frontmatter/00-index.md) |
| 本地引用 | 当前文档相对路径与 `/` 开头的当前 root 相对路径；检查行为见 [本地目标演示](docs/demos/checkers/02-local-target-missing.md) |
| 行内扩展 | `==高亮==`、`^^插入^^` 与 [`++ctrl+alt+del++` 键盘按键](docs/demos/pymarkdown/keyboards.md) |
| 协作标记 | [Critic Markup 行内与块级语法](docs/demos/pymarkdown/critic.md) |
| WebUI 增强 | 代码复制、行号、长代码块折叠、跨站链接新标签页打开，以及图片和 Mermaid Lightbox（支持滚轮平滑缩放） |

## 许可证

[MIT](LICENSE)
