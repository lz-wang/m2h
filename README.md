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
m2h docs
m2h convert README.md
```

使用 `m2h --version` 查看当前版本。

## 转换为 HTML

```console
# 转换单个文件，生成完全自包含的 README.html
m2h convert README.md

# 指定输出文件
m2h convert README.md --output public/index.html

# 转换目录中的 Markdown，并保留目录结构
m2h convert docs --output public/docs --depth 3 --glob '**/plan_*.md'

# 目录转换时让每个 HTML 也自包含
m2h convert docs --standalone
```

单文件转换默认生成完全自包含的 HTML：页面样式、数学公式与 Mermaid 运行时、字体以及本地图片全部内嵌，输出旁不再生成 `.m2h/` 目录，文件可独立离线打开与分享；文档没有公式或图表时不会内嵌对应运行时。目录转换会在输出根部生成共享的 `.m2h/` 运行时目录，并默认复制非 Markdown 资源（`--copy-assets=false` 关闭）；使用 `--standalone` 可让目录中每个 HTML 也自包含。相对 Markdown 链接会改为对应的 `.html` 链接。

执行转换前，`m2h` 会在终端显示写入目标并要求确认（`[y/N]`，直接回车取消，仅 `y`/`yes` 继续）；加 `--yes` 或 `-y` 跳过确认。标准输入不是终端时（脚本、CI、管道）不加 `--yes` 会直接报错退出。`m2h` 文档服务不受影响。

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
m2h README.md

# 预览目录；提供文件树、搜索、分享、主题、正文宽度、文档目录与右下角回到顶部/前往底部按钮
m2h docs --mode dark --width wide

# 同时预览多个文件或目录（逗号分隔与空格分隔等价）
m2h docs,wiki
m2h docs wiki notes.md
```

服务默认监听 `http://127.0.0.1:8793` 并自动打开浏览器，按 `Ctrl+C` 停止服务；加 `--no-open` 可仅启动服务。

多输入预览在侧边栏以并列根展示（以输入名标注，重名自动加序号，根默认展开），每个根的文档与资源相互隔离：文档地址带根编号前缀（如 `/doc/r0/README.md`、`/assets/r0/images/logo.png`），两个根中的同名文档、同名图片互不混淆，Markdown 中爬出所在根的相对链接会被安全拒绝。第一个输入是主根，决定默认打开的文档；搜索覆盖全部根，按根名搜索会列出该根下所有文档；`--glob` 与 `--depth` 作用于全部目录输入（纯单文件输入时仍不可用）。同一目录树的重复输入（含符号链接别名）会被拒绝。单输入预览的地址与行为保持不变。

每个文档同时提供两种地址：`/doc/<路径>` 返回渲染后的预览页，`/raw/<路径>`（多根预览同样带根编号前缀，如 `/raw/r0/docs/guide.md`）直接返回原始 Markdown 源文件（`text/markdown; charset=utf-8`，包含 frontmatter，支持 GET 与 HEAD）。两种地址共享同一可见性边界：越界路径、未知根、未通过 `--glob`/`--depth` 筛选的文件与非 Markdown 文件一律被拒绝。

工具栏的分享按钮（位于宽度调整左侧）可复制当前文档的四种信息：文档网页链接（`/doc/<路径>`，保留当前标题位置 hash、不携带主题/宽度等个人界面参数）、文档本地路径（服务器上的绝对路径，单文件输入不会重复拼接文件名）、Markdown 链接（`/raw/<路径>`）与 Markdown 全文（点击时才按需获取原始源文件，包含 frontmatter）。复制成功后底部显示短暂的状态提示。

侧边栏文件树支持右键菜单：文件行提供“新页面打开”（新标签页打开渲染页）与复制文档网页链接、文档本地路径、Markdown 链接；文件夹行与多根预览的根行提供“复制文件夹本地路径”（仅目录输入的根提供）。右键不会改变当前打开的文档、折叠状态或阅读位置，左键行为保持不变。

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
| 语法高亮        | 围栏代码块根据语言标识进行高亮，并提供复制按钮；Web 预览的代码块左侧显示行号，横向滚动时行号与复制按钮位置保持固定，复制仍只复制源码；超过 25 行的代码块默认折叠，点击可展开完整内容 |
| 扩展行内标记      | `==高亮==`、`^^插入^^`、键盘按键 `++ctrl+alt+del++` |
| Critic 协作标记 | `{==高亮==}`、`{--删除--}`、`{++新增++}`、`{~~旧~>新~~}` |
| 数学公式        | `$E = mc^2$`；行间公式使用 `$$...$$`  |
| Mermaid 图表  | 使用 ` ```mermaid ` 围栏定义图表，配色随页面主题（浅色/深色）自动切换 |
| 可排序表格       | 普通 Markdown 表格点击表头即按该列排序，支持键盘操作 |
| Frontmatter | 支持读取 Markdown YAML frontmatter |
| 文档目录        | Web 预览可根据标题生成 TOC              |
| 图片大图查看     | Web 预览正文图片右上角放大按钮进入全屏查看，支持切换、缩放、旋转 |

数学公式、Mermaid 图表与表格排序的运行时资源已包含在输出中，生成的 HTML 可离线打开。

扩展行内标记借鉴 [PyMdown Extensions](https://facelessuser.github.io/pymdown-extensions/) 的语义：`==文本==` 渲染为 `<mark>`，`^^文本^^` 渲染为 `<ins>`，`++ctrl+alt+del++` 渲染为带样式的键盘按键。按键数据库与 PyMdown 的 English US 键盘一致（字母、数字、标点、导航、编辑、数字小键盘、修饰键、F1–F24、媒体、浏览器与鼠标按键），别名（如 `ctrl`、`cmd`、`pg-up`、`pipe`）统一归一化为标准键名并输出同一 CSS class，修饰键与导航键在键帽上显示对应符号，未知按键保留原文。Critic 协作标记用 `{ ... }` 包裹，支持高亮、删除、新增、备注与替换（`{~~旧~>新~~}` 渲染为 `<del>` 加 `<ins>`）；把 `{==`、`{++` 或 `{--` 单独写在一行，再用 `==}`、`++}` 或 `--}` 结束，即可标注整段内容，段内仍按正常 Markdown 解析。

## 许可证

[MIT](LICENSE)
