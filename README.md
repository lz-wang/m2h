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

`--enable` 与 `--disable` 选择要运行的规则：默认规则之上追加、或从中移除
（逗号分隔多个规则，`--disable` 优先于 `--enable`；`all` 指代全部规则）。
表中标"关"的规则因误报场景客观存在，仅经 `--enable` 显式开启：

```console
m2h check docs --enable section.empty,unicode.mojibake
m2h check docs --disable image.alt-empty
m2h check docs --enable all --disable image.alt-empty
```

`--disable` 始终优先，因此 `--disable all --enable <规则>` 的实际结果是
一条规则都不运行，而不是只运行该规则。

未知规则名在读取任何文件之前即失败：

```text
Error: unknown check rule "foo.bar"
```

检查规则（25 条）：

| 规则 | 等级 | 默认 | 说明 |
| --- | --- | --- | --- |
| `frontmatter.invalid` | error | 开 | Frontmatter YAML 无法解析或根节点不是 mapping |
| `local-target.missing` | error | 开 | 本地链接、图片或附件目标不存在，或经 `/assets` 路由不可达（如指向 `.md` 的图片） |
| `local-target.not-regular` | error | 开 | 引用目标存在，但不是普通文件 |
| `local-target.outside-root` | error | 开 | `../` 或 symlink 使目标越过文档根目录 |
| `markdown-target.not-served` | error | 开 | Markdown 目标存在，但被单文件模式或 `--glob`/`--depth` 排除 |
| `anchor.missing` | error | 开 | `#anchor` 或 `foo.md#anchor` 指向不存在的标题锚点 |
| `reference.undefined` | error | 开 | `[text][label]` 或 `[text][]` 引用了不存在的 reference 定义；裸 `[label]` 不报 |
| `footnote.undefined` | error | 开 | `[^label]` 没有对应定义（按渲染器同一逐字节比较） |
| `footnote.empty` | error | 开 | 脚注定义没有任何内容（多行缩进续行不算空） |
| `table.column-mismatch` | error | 开 | 表格行列数与分隔行不一致（渲染时会被补空或截断），或表头与分隔行列数不符导致整表被拒绝 |
| `html.comment-unclosed` | error | 开 | `<!--` 没有闭合，其后内容整体渲染为注释 |
| `link.reversed` | error | 开 | 高置信度识别 `(text)[url]` 反转链接写法（destination 须为 scheme、路径或已知文件扩展名）；`f(x)[0]`、`array[index]`、`(version)[v1.2]` 不报 |
| `image.alt-empty` | warning | 开 | 图片没有 alt 文本 |
| `document.multiple-h1` | warning | 开 | 一个文档包含多个 H1 |
| `heading.level-skip` | warning | 开 | 标题层级向下跳超过一级（向上跳任意级合法） |
| `heading.duplicate` | warning | 开 | 同一父 section 下出现重复标题文本；不同 section 的同名标题合法 |
| `code-fence.language-missing` | warning | 开 | fenced code 未指定语言，含 blockquote/列表内的 fence（由解析器 AST 判定；indented code 与 raw HTML 块内的 ``` 不检查） |
| `footnote.unused` | warning | 开 | 脚注定义从未被引用 |
| `reference.unused` | warning | 开 | reference 定义从未被引用 |
| `frontmatter.date-invalid` | warning | 开 | 已识别的日期字段不是有效 ISO 日期 |
| `link.empty-destination` | warning | 开 | 链接或图片的 destination 为空 |
| `section.empty` | warning | **关** | 标题与下一个标题之间没有任何渲染内容（父标题只承载子标题是常见合法结构） |
| `link.text-nondescriptive` | warning | **关** | 链接文本为 `click here`/`点击这里` 等无信息短语（仅精确匹配，仅 Markdown 链接） |
| `unicode.mojibake` | warning | **关** | 疑似错误编码文本（`CafÃ©`、`â€™` 等多字符签名；单字符不报） |
| `unicode.invisible-character` | warning | **关** | 可疑不可见字符（仅行首/行尾、邻接空白、连续出现或 bidi 控制字符触发；emoji 的 ZWJ 与 variation selector 不报） |

说明：

- 只检查相对本地引用；`https://`、`mailto:`、`tel:`、`//cdn.example.com`
  与绝对路径默认跳过
- Markdown 链接/图片、reference-style 链接与 raw HTML 的
  `href`/`src`/`poster`/`data` 均在检查范围内
- 与 Web 浏览行为严格一致：同一 Markdown 解析引擎、同一 URL 解码与
  `/doc`/`/assets` 路由判定（Markdown 文件不会经 `/assets` 提供）、同一
  文档范围与 symlink 安全边界、同一 GitHub 兼容锚点算法
- 引用与脚注的"未定义/未使用"判定由实际解析器给出（reference 标签按
  渲染器同一归一化比较、脚注标签逐字节比较），行内与 fenced 代码、
  raw HTML 与 HTML 注释（含 `<code>`、`<kbd>` 等字面内容元素）中的
  内容永不参与语法判定；`unicode.*` 检查的是源文件质量，仍扫描
  raw HTML，仅代码区域豁免；所有正文诊断的行列号已含 Frontmatter 偏移
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
