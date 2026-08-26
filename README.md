<p align="center">
  <img src="web/public/favicon.svg" alt="m2h Logo" width="120" height="120">
</p>

[![codecov](https://codecov.io/gh/lz-wang/m2h/graph/badge.svg?token=iNo6LuOlzm)](https://codecov.io/gh/lz-wang/m2h)

# m2h

`m2h` 是一个轻量、零配置的 Markdown Web 文档浏览与服务工具。直接指定 Markdown 文件或目录即可启动内嵌 WebUI，支持 GFM、数学公式、Mermaid、文件树导航与实时刷新等能力；同时提供简单的单文件 HTML 导出功能。

## 安装

### Homebrew

macOS 和 Linux：

```console
brew install lz-wang/tap/m2h
```

升级已安装的版本：

```console
brew upgrade m2h
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
# 在浏览器中打开 docs 目录的文档服务
m2h docs

# 也可以直接打开单个 Markdown 文件
m2h README.md
```

使用 `m2h --version` 查看当前版本，`m2h --help` 查看全部选项。

## 启动文档服务

```console
# 打开单个文件；不显示文件侧边栏，保存后自动刷新正文
m2h README.md

# 打开目录；提供文件树、搜索、分享、主题、正文宽度、文档目录与右下角回到顶部/前往底部按钮
m2h docs --mode dark --width wide

# 同时打开多个文件或目录（逗号分隔与空格分隔等价）
m2h docs,wiki
m2h docs wiki notes.md

# 在 VPS 等远程机器上对外提供服务
m2h docs --host 0.0.0.0 --no-open
```

服务默认监听 `http://127.0.0.1:8793` 并自动打开浏览器，按 `Ctrl+C` 停止服务；加 `--no-open` 可仅启动服务。

`m2h` 会监听全部输入的文件变化（目录输入递归监听，不跟随目录内符号链接）：修改、新增或删除 Markdown 文件后，文件树与当前正文自动刷新，当前打开的文档被删除时回到未选择状态。

根地址 `/` 表示工作区本身：目录与多根输入启动后不自动打开任何文档，正文显示“请选择要查看的文件”，从文件树选择后进入 `/doc/<路径>`；直接访问 `/doc/<路径>` 深链接仍立即打开对应文档。单文件输入没有文件树可选，仍自动打开唯一文件。

服务器本地路径不进入 WebUI：`/api/files` 只返回逻辑文档信息，任何监听地址下都不携带根的绝对路径，WebUI 也不提供本地路径的悬停提示与复制入口。

多根工作区在侧边栏以并列根展示（以输入名标注，重名自动加序号），每个根的文档与资源相互隔离：文档地址带根编号前缀（如 `/doc/r0/README.md`、`/assets/r0/images/logo.png`），两个根中的同名文档、同名图片互不混淆，Markdown 中爬出所在根的相对链接会被安全拒绝。文件树初始只展开选中文档所在的根及其上级目录；未选择文档时单根输入展开第一层目录、多根输入全部折叠。搜索覆盖全部根，按根名搜索会列出该根下所有文档；`--glob` 与 `--depth` 作用于全部目录输入（纯单文件输入时仍不可用）。同一目录树的重复输入（含符号链接别名）会被拒绝。单输入的地址与行为保持不变。

每个文档同时提供两种地址：`/doc/<路径>` 返回渲染后的阅读页，`/raw/<路径>`（多根工作区同样带根编号前缀，如 `/raw/r0/docs/guide.md`）直接返回原始 Markdown 源文件（`text/markdown; charset=utf-8`，包含 frontmatter，支持 GET 与 HEAD）。两种地址共享同一可见性边界：越界路径、未知根、未通过 `--glob`/`--depth` 筛选的文件与非 Markdown 文件一律被拒绝。

工具栏的分享按钮（位于宽度调整左侧）可复制当前文档的三种信息：文档网页链接（`/doc/<路径>`，保留当前标题位置 hash、不携带主题/宽度等个人界面参数）、Markdown 链接（`/raw/<路径>`）与 Markdown 全文（点击时才按需获取原始源文件，包含 frontmatter）。复制成功后底部显示短暂的状态提示。

侧边栏文件树支持右键菜单：文件行提供“新页面打开”（新标签页打开渲染页）、复制文档网页链接与复制 Markdown 链接；目录行不提供右键菜单，保持左键展开/折叠。右键不会改变当前打开的文档、折叠状态或阅读位置，左键行为保持不变。

正文中的普通图片（含图片链接与原始 HTML 中的单图 `<img>` 与 `<picture>`，`<picture>` 保留源选择语义；一个链接内含多张图片时整体不提供放大按钮，Mermaid 图表除外）右上角提供大图查看入口，点击进入全屏 Lightbox：底部半透明工具栏支持在当前文档的图片间上一张/下一张切换（按图片在正文中的当前顺序编号，表格排序后仍与实际图片对应，首尾不循环）、缩放（1–5 倍，放大后可拖动查看，拖动范围限制在可视区域内）、±90° 旋转（旋转后按预留工具栏的可用区域重新适配）与位置计数，键盘 `←`/`→` 切换图片；点击图片本体仍保持原有行为（图片链接继续跟随链接），放大按钮位于链接之外，不产生交互元素嵌套；关闭按钮、`Esc` 与点击遮罩空白处均可退出，关闭后正文阅读位置与地址栏锚点保持不变。

| 选项 | 说明 |
| --- | --- |
| `--host` | 监听地址；默认 `127.0.0.1`。 |
| `--port`, `-p` | 监听端口；默认 `8793`。 |
| `--open` / `--no-open` | 启动后是否打开系统默认浏览器；默认打开，使用 `--no-open` 关闭。 |
| `--mode` | 页面主题：`light`、`dark` 或 `auto`；默认 `auto`。 |
| `--width` | 正文宽度：`standard`、`wide` 或 `full`；默认 `standard`。 |
| `--toc` | 是否显示文档目录；默认 `true`，关闭使用 `--toc=false`。 |
| `--glob` | 按相对各目录输入的路径筛选 Markdown，作用于全部目录输入。 |
| `--depth`, `-d` | 最大递归深度；默认 `4`。作用于全部目录输入。 |

## 导出 HTML

除文档服务外，`m2h convert` 可以把单个 Markdown 文件导出为一个 HTML 文件：

```console
# 导出单个文件，在源文件旁生成 README.html
m2h convert README.md

# 指定输出文件名（只接受文件名，始终写入源文件所在目录）
m2h convert README.md -o index.html
```

导出输出为单个 HTML 文件：m2h 的 Markdown 页面样式直接内联；KaTeX、Mermaid 与表格排序运行时按需从 jsDelivr CDN 加载固定版本（与 WebUI 内嵌运行时一致；表格排序仅加载核心脚本，类型化比较器只在 WebUI 提供），没有公式、图表或表格的文档不会加载任何大型脚本，查看公式与图表需要网络连接。本地图片与相对 Markdown 链接保留原始相对路径，输出 HTML 与 Markdown 源文件放在同一目录即可正常引用图片与相互链接。

输出文件已存在时导出会报错退出；加 `--force` 覆盖已有输出，适合脚本与 CI：

```console
m2h convert README.md --force
```

导出成功后，m2h 会向标准输出打印生成的 HTML 绝对路径：

```text
Wrote /work/project/README.html
```

导出的 HTML 保证 Markdown 正确渲染、语法高亮、GitHub 风格样式、数学公式、Mermaid 图表、可排序表格与标题锚点；不包含 WebUI 独有的交互（Lightbox、代码行号、代码折叠、分享等）。

| 选项 | 说明 |
| --- | --- |
| `--output`, `-o` | 输出 HTML 文件名（不含目录），写入 Markdown 源文件所在目录；默认同名 `.html`。 |
| `--mode` | 页面主题：`light`、`dark` 或 `auto`；默认 `auto`。 |
| `--width` | 正文宽度：`standard`（980px）、`wide`（1280px）或 `full`；默认 `standard`。 |
| `--force` | 输出文件已存在时覆盖，而不是报错退出。 |

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
| 表格      | 使用 `\|` 定义表格列，渲染后表头可点击排序 |
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
| 语法高亮        | 围栏代码块根据语言标识进行高亮；WebUI 额外提供复制按钮与行号（横向滚动时行号与复制按钮位置保持固定，复制只复制源码），超过 25 行的代码块默认折叠，点击可展开完整内容 |
| 扩展行内标记      | `==高亮==`、`^^插入^^`、键盘按键 `++ctrl+alt+del++` |
| Critic 协作标记 | `{==高亮==}`、`{--删除--}`、`{++新增++}`、`{~~旧~>新~~}` |
| 数学公式        | `$E = mc^2$`；行间公式使用 `$$...$$`  |
| Mermaid 图表  | 使用 ` ```mermaid ` 围栏定义图表，配色随页面主题（浅色/深色）自动切换 |
| 可排序表格       | 普通 Markdown 表格点击表头即按该列排序，支持键盘操作 |
| Frontmatter | 支持读取 Markdown YAML frontmatter |
| 文档目录        | WebUI 可根据标题生成 TOC              |
| 图片大图查看     | WebUI 正文图片右上角放大按钮进入全屏查看，支持切换、缩放、旋转 |

数学公式、Mermaid 图表与表格排序的运行时在 WebUI 中内嵌于二进制、离线可用；HTML 导出按需从 jsDelivr CDN 加载固定版本的同一组运行时，其中表格排序仅加载核心脚本（按核心默认规则比较），日期、文件大小等类型化比较器只在 WebUI 提供。

扩展行内标记借鉴 [PyMdown Extensions](https://facelessuser.github.io/pymdown-extensions/) 的语义：`==文本==` 渲染为 `<mark>`，`^^文本^^` 渲染为 `<ins>`，`++ctrl+alt+del++` 渲染为带样式的键盘按键。按键数据库与 PyMdown 的 English US 键盘一致（字母、数字、标点、导航、编辑、数字小键盘、修饰键、F1–F24、媒体、浏览器与鼠标按键），别名（如 `ctrl`、`cmd`、`pg-up`、`pipe`）统一归一化为标准键名并输出同一 CSS class，修饰键与导航键在键帽上显示对应符号，未知按键保留原文。Critic 协作标记用 `{ ... }` 包裹，支持高亮、删除、新增、备注与替换（`{~~旧~>新~~}` 渲染为 `<del>` 加 `<ins>`）；把 `{==`、`{++` 或 `{--` 单独写在一行，再用 `==}`、`++}` 或 `--}` 结束，即可标注整段内容，段内仍按正常 Markdown 解析。

## 许可证

[MIT](LICENSE)
