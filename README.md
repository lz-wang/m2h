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
- 图片与 Mermaid 图表支持 Lightbox 查看，可切换、缩放、拖动和旋转
- 文件修改后重新打开即可读取最新内容，刷新页面可重新扫描目录
- 可将单个 Markdown 文件导出为 HTML

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

## Markdown 支持

m2h 支持常用 GFM Markdown，并提供以下扩展：

* GitHub 风格语法高亮与标题锚点
* 脚注、Emoji、GitHub Alerts
* 数学公式与 Mermaid（含按需加载的 ZenUML 时序图）
* YAML Frontmatter
* 可排序表格
* `==高亮==`、`^^插入^^`
* `++ctrl+alt+del++` 键盘按键
* Critic Markup
* WebUI 代码复制、行号与长代码块折叠
* WebUI 图片 Lightbox

## 许可证

[MIT](LICENSE)
