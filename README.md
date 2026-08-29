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
- 支持 Frontmatter（`title` 字段作为文档展示标题，`create_date`/`create_at`/`create_time` 与 `update_date`/`update_at`/`update_time` 别名按优先级归一为创建/更新时间摘要，`date` 为兜底）、可排序表格、代码行号与长代码块折叠
- 图片与 Mermaid 图表支持 Lightbox 查看，可切换、通过工具栏或鼠标滚轮平滑缩放、拖动和旋转
- 文件修改后重新打开即可读取最新内容，刷新页面可重新扫描目录
- 可将单个 Markdown 文件导出为 HTML
- 可检查 Markdown 文档的 Frontmatter、本地引用、锚点与结构问题

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

远程提供文档服务：

```console
m2h docs --host 0.0.0.0 --no-open
```

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

检查单个文件或目录的完整性问题：

```console
m2h check README.md
m2h check docs
```

输出遵循 `path:line:column` 约定，便于终端与 IDE 定位：

```text
docs/guide.md:42:17: error [local-target.missing]: target "images/topology.png" does not exist
docs/index.md:18:5: error [anchor.missing]: heading "#installation" does not exist in "guide.md"
docs/logo.md:12:1: warning [image.alt-empty]: image has no alt text

Checked 27 Markdown files: 2 errors, 1 warning
```

`--depth` 与 `--glob` 和浏览命令一致，因此 `m2h docs` 与 `m2h check docs`
看到的是同一批文档；`--format json` 输出结构化结果，`--strict` 把
warning 也视为失败：

```console
m2h check docs --depth 8 --glob '**/*.md'
m2h check docs --format json
m2h check docs --strict
```

检查规则：

| 规则 | 等级 | 说明 |
| --- | --- | --- |
| `frontmatter.invalid` | error | Frontmatter YAML 无法解析或根节点不是 mapping |
| `local-target.missing` | error | 本地链接、图片或附件目标不存在 |
| `local-target.not-regular` | error | 引用目标存在，但不是普通文件 |
| `local-target.outside-root` | error | `../` 或 symlink 使目标越过文档根目录 |
| `markdown-target.not-served` | error | Markdown 目标存在，但被单文件模式或 `--glob`/`--depth` 排除 |
| `anchor.missing` | error | `#anchor` 或 `foo.md#anchor` 指向不存在的标题锚点 |
| `image.alt-empty` | warning | 图片没有 alt 文本 |
| `document.multiple-h1` | warning | 一个文档包含多个 H1 |
| `frontmatter.date-invalid` | warning | 已识别的日期字段不是有效 ISO 日期 |
| `link.empty-destination` | warning | 链接或图片的 destination 为空 |

说明：

- 只检查相对本地引用；`https://`、`mailto:`、`tel:`、`//cdn.example.com`
  与绝对路径默认跳过
- Markdown 链接/图片、reference-style 链接与 raw HTML 的
  `href`/`src`/`poster`/`data` 均在检查范围内
- 与 Web 浏览行为严格一致：同一 Markdown 解析引擎、同一文档范围与
  symlink 安全边界、同一 GitHub 兼容锚点算法
- 退出码：发现 error（或 `--strict` 下存在 warning）时返回 `1`

查看全部选项：

```console
m2h check --help
```

## Markdown 支持

m2h 支持常用 GFM Markdown，并提供以下扩展：

* GitHub 风格语法高亮与标题锚点
* 脚注、Emoji、GitHub Alerts
* 数学公式（`$...$` 行内、`$$...$$` 行间；`$9`、`$200` 等金额保持普通文本）与 Mermaid（含按需加载、隔离宿主样式并响应亮暗模式的 ZenUML 时序图）
* YAML Frontmatter
* 可排序表格
* `==高亮==`、`^^插入^^`
* `++ctrl+alt+del++` 键盘按键
* Critic Markup
* WebUI 代码复制、行号与长代码块折叠
* WebUI 图片与 Mermaid 图表 Lightbox（支持鼠标滚轮平滑缩放）

## 许可证

[MIT](LICENSE)
