<p align="center">
  <img src="web/public/favicon.svg" alt="m2h Logo" width="120" height="120">
</p>

# m2h

[![codecov](https://codecov.io/gh/lz-wang/m2h/graph/badge.svg?token=iNo6LuOlzm)](https://codecov.io/gh/lz-wang/m2h)

`m2h` 是一个 Markdown 浏览、标注、检查与 HTML 导出工具。

## 特性

- 浏览单个 Markdown 文件、目录或多个文档目录。
- 文档首页、文件树筛选、全文搜索与匹配章节跳转。
- 文档目录、明暗主题与正文宽度切换。
- GFM、语法高亮、数学公式、Mermaid、Frontmatter 与扩展语法。
- 可排序表格、代码复制、行号与长代码块折叠。
- 图片与图表全屏查看，支持缩放、拖动和旋转。
- 单个 Markdown 文件导出为 HTML。
- 登录后高亮、评论、定位与导出标注，支持分享给 Agent。
- 检查 Frontmatter、本地引用、锚点与文档结构。

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

打开文件、目录或多个文档目录：

```console
m2h README.md
m2h docs
m2h docs wiki notes.md
```

默认在 `127.0.0.1:8793` 启动并打开浏览器。按 **Ctrl/Cmd + K** 打开全局搜索。

导出单个文件为同名 HTML：

```console
m2h export README.md
```

更多用法见 [浏览与导出 Markdown](docs/browsing.md)，全部选项见 `m2h --help`。

### 标注 Markdown

启动时启用本地认证。

```console
M2H_LOCAL_USERS_JSON='{"alice":"请替换为你的密码"}' m2h docs --auth local
```

登录后，选中正文并点击「标注」。可选琥珀、绿、红、紫、蓝五色，默认琥珀色。
LDAP 配置见 [内置认证](docs/deployment.md#内置认证可选v0270-起)。

### 公开分享文档

在启用认证的服务上添加 `--public-shares`，登录后打开文档并点击「分享文档」。持链接者无需登录，可阅读文档的当前内容。默认有效期为 30 分钟。

启用方式及公开范围见 [公开分享](docs/deployment.md#公开分享)。

### 检查 Markdown

检查文件或目录：

```console
m2h check README.md
m2h check docs
```

输出问题位置与规则名称，发现错误时退出码为 `1`。
高级选项与 32 条规则见 [检查指南与规则演示](docs/demos/checkers/00-index.md)。

### 高级主题

- [VPS 部署](docs/deployment.md)
- [.gitignore 过滤](docs/gitignore.md)
- [隐藏文件发布](docs/hidden.md)
- [全文搜索一致性](docs/search.md)

## Markdown 支持

| 类别 | 支持内容与演示 |
| --- | --- |
| GitHub 风格 | GFM、[语法高亮](docs/demos/syntax-highlight/00-index.md)、标题锚点、脚注、Emoji、GitHub Alerts、[单行表头与表格排序](docs/demos/table-layout.md) |
| 富内容 | [数学公式](docs/demos/math.md)、[Mermaid 图表与浏览器兼容要求](docs/demos/mermaid/00-index.md) |
| 元数据 | [Frontmatter 标题、日期与描述](docs/demos/frontmatter/00-index.md) |
| 本地引用 | [文档、图片与目录链接](docs/demos/checkers/02-local-target-missing.md) |
| 行内扩展 | `==高亮==`、`^^插入^^`、[键盘按键](docs/demos/pymarkdown/keyboards.md) |
| 标签与协作 | [标签分组](docs/demos/pymarkdown/tabbed.md)、[Critic Markup](docs/demos/pymarkdown/critic.md) |
| 图片 | 图片居中、延迟加载、全屏查看与 [SVG 文字选择](docs/demos/svg-images.md) |

## 许可证

[MIT](LICENSE)
