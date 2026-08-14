<p align="center">
  <img src="web/public/favicon.svg" alt="m2h Logo" width="120" height="120">
</p>

[![codecov](https://codecov.io/gh/lz-wang/m2h/graph/badge.svg?token=iNo6LuOlzm)](https://codecov.io/gh/lz-wang/m2h)

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
# 转换单个文件，生成完全自包含的 README.html
$ m2h README.md

# 指定输出文件
$ m2h README.md --output public/index.html

# 转换目录中的 Markdown，并保留目录结构
$ m2h docs --output public/docs --depth 3 --glob '**/plan_*.md'

# 目录转换时让每个 HTML 也自包含
$ m2h docs --standalone
```

单文件转换默认生成完全自包含的 HTML：页面样式、数学公式与 Mermaid 运行时、字体以及本地图片全部内嵌，输出旁不再生成 `.m2h/` 目录，文件可独立离线打开与分享；文档没有公式或图表时不会内嵌对应运行时。目录转换会在输出根部生成共享的 `.m2h/` 运行时目录，并默认复制非 Markdown 资源（`--copy-assets=false` 关闭）；使用 `--standalone` 可让目录中每个 HTML 也自包含。相对 Markdown 链接会改为对应的 `.html` 链接。

执行转换前，`m2h` 会在终端显示写入目标并要求确认（`[y/N]`，直接回车取消，仅 `y`/`yes` 继续）；加 `--yes` 或 `-y` 跳过确认。标准输入不是终端时（脚本、CI、管道）不加 `--yes` 会直接报错退出。`m2h web` 预览不受影响。

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
| `--standalone` | 目录模式下把运行时与本地图片内嵌进每个 HTML；单文件转换默认已自包含。仅目录可用。 |
| `--copy-assets` | 是否复制非 Markdown 资源；默认 `true`。 |
| `--mode` | 页面主题：`light`、`dark` 或 `auto`；默认 `auto`。 |
| `--width` | 正文宽度：`standard`（980px）、`wide`（1280px）或 `full`；默认 `standard`。 |
| `--yes`, `-y` | 跳过转换前的确认提示；非交互环境（脚本、CI、管道）必须加此选项。 |

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

## Markdown 兼容性

### GFM

m2h 基于 [Goldmark](https://github.com/yuin/goldmark) 解析 Markdown，支持 [GitHub Flavored Markdown](https://github.github.com/gfm/) 的常用语法。

| 特性      | 简单示例                                                 |         
| ------- | ---------------------------------------------------- | 
| 标题      | `## 安装`                                              |         
| 强调与删除线  | `**加粗**`、`*斜体*`、`~~删除~~`                             |         
| 列表与任务列表 | `- 项目`、`1. 第一步`、`- [x] 已完成`                          |         
| 引用与分割线  | `> 引用内容`、`---`                                       |         
| 链接与图片   | `[网站](https://example.com)`、`![图片](images/demo.png)` |         
| 自动链接    | `<https://example.com>` 或 `https://example.com`      |         
| 行内与围栏代码 | `` `go test` ``；围栏以 ` ```go ` 开始、以 ` ``` ` 结束        |         
| 表格      | 使用 `\|` 定义表格列 |
| 原始 HTML | 例如 `<details>...</details>`                          |         

### GitHub 扩展

除 GFM 外，m2h 还支持部分 GitHub Markdown 常用扩展。

| 特性            | 简单示例                            |
| ------------- | ------------------------------- |
| 标题锚点          | `## 安装` 可通过 `#安装` 链接；重复标题自动添加序号 |
| 脚注            | `说明[^1]`，并在文末写 `[^1]: 补充说明`     |
| Emoji 短代码     | `:rocket:` → 🚀                 |
| GitHub Alerts | `> [!NOTE]`、`> [!WARNING]` 等    |

### m2h 扩展

m2h 在 Markdown 渲染基础上提供以下增强能力。

| 特性          | 简单示例                           |
| ----------- | ------------------------------ |
| 语法高亮        | 围栏代码块根据语言标识进行高亮，并提供复制按钮        |
| 扩展行内标记      | `==高亮==`、`^^插入^^`、键盘按键 `++ctrl+alt+del++` |
| Critic 协作标记 | `{==高亮==}`、`{--删除--}`、`{++新增++}`、`{~~旧~>新~~}` |
| 数学公式        | `$E = mc^2$`；行间公式使用 `$$...$$`  |
| Mermaid 图表  | 使用 ` ```mermaid ` 围栏定义图表，配色随页面主题（浅色/深色）自动切换 |
| Frontmatter | 支持读取 Markdown YAML frontmatter |
| 文档目录        | Web 预览可根据标题生成 TOC              |

数学公式和 Mermaid 图表的运行时资源已包含在输出中，生成的 HTML 可离线打开。

扩展行内标记借鉴 [PyMdown Extensions](https://facelessuser.github.io/pymdown-extensions/) 的语义：`==文本==` 渲染为 `<mark>`，`^^文本^^` 渲染为 `<ins>`，`++ctrl+alt+del++` 渲染为带样式的键盘按键。按键数据库与 PyMdown 的 English US 键盘一致（字母、数字、标点、导航、编辑、数字小键盘、修饰键、F1–F24、媒体、浏览器与鼠标按键），别名（如 `ctrl`、`cmd`、`pg-up`、`pipe`）统一归一化为标准键名并输出同一 CSS class，修饰键与导航键在键帽上显示对应符号，未知按键保留原文。Critic 协作标记用 `{ ... }` 包裹，支持高亮、删除、新增、备注与替换（`{~~旧~>新~~}` 渲染为 `<del>` 加 `<ins>`）；把 `{==`、`{++` 或 `{--` 单独写在一行，再用 `==}`、`++}` 或 `--}` 结束，即可标注整段内容，段内仍按正常 Markdown 解析。

## 许可证

[MIT](LICENSE)
