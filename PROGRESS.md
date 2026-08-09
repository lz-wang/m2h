# m2h 开发进度

最后更新：2026-08-10

## 当前状态

- 当前里程碑：阶段 4 已完成，下一步是阶段 5“目录 `preview` API 与安全边界”。
- 当前可用能力：完整构建与测试工具链、m2h CLI 帮助、版本命令、共享 GFM 渲染核心、完整 `convert`，以及具备父目录监听、SSE 实时刷新、安全附件路由和优雅关闭的单文件 `preview`；目录 `preview` 与 `view` 尚未接入。
- 已完成提交：
  - `ab493ce docs(init): 初始化项目基础文档`
  - `f7e9552 chore(init): 初始化项目工程骨架`
  - `b18482e feat(cli): 完成阶段 1 工具链与版本骨架`
  - `7f56e5d feat(markdown): 完成阶段 2 共享渲染核心`
  - `171796a feat(convert): 完成阶段 3 文件发现与转换`
- CI 状态：阶段 3 的 main 与 `v0.3.0` release workflow 已通过测试、Codecov、六平台构建、Artifacts、WebDAV、GitHub Release 与通知；v0.3.0 的 12 个 Release 资产、正文和 Darwin arm64 实际转换已核验。

本文件是实现顺序、范围和验收状态的唯一进度来源。用户命令契约见 `README.md`，发布摘要见 `CHANGELOG.md`。

## 产品目标

交付一个跨平台 Go CLI，将单个或一组 Markdown 转换为 GitHub 风格 HTML，在浏览器中预览文件或目录，并在终端中渲染单个 Markdown。`convert` 与 `preview` 必须共享同一个 `internal/markdown` 核心，确保语法、主题、安全与链接规则一致。

## 本轮初始化结果

- [x] 创建 100 行以内、渐进式披露的 `AGENTS.md`。
- [x] 创建面向 CLI 用户的 `README.md`，并明确功能尚未实现。
- [x] 创建只记录新特性与修复的 Keep a Changelog 格式 `CHANGELOG.md`。
- [x] 创建 Makefile、`.gitignore`、六平台 build/release、Codecov、WebDAV、Pushover workflows。
- [x] 用 `.gitkeep` 初始化 `internal/*` 与 `web/src` 目录。
- [x] 将后续实现拆成有依赖、有测试、有完成标准的阶段。
- [x] 本轮没有创建 `main.go`、模块清单、前端清单或业务实现。

## 已确定的产品契约

### 命令与输入

- 对外命令统一为 `version`、`convert`、`preview`、`view`；需求中的 `serve` 均解释为 `preview`，不额外提供 `serve` 别名。
- `docs` 与 `docs/` 必须解析为相同的目录输入。
- 未知参数报 `Error: unknown option`；存在但不适用于输入类型的参数返回针对性错误，并使用非零退出码。
- 所有枚举参数先校验再访问文件系统；`--mode` 只接受 `light`、`dark`、`auto`。

### 路径、glob 与符号链接

- 目录枚举结果和 API 路径都是相对于输入根目录、以 `/` 分隔的 clean path。
- `--depth 2` 包含根目录文件（深度 0）、一级子目录（深度 1）和二级子目录（深度 2）。
- doublestar glob 在深度过滤后匹配相对路径；同一规则用于 `convert` 与目录 `preview`。
- 输入本身是文件或目录符号链接时允许解析目标；根目录内部的符号链接目录不递归跟随。
- 根目录内部的符号链接文件只有在解析目标仍位于根目录内时才允许读取；越界目标拒绝或跳过，并写入可诊断日志。
- API 和静态资源访问必须拒绝绝对路径、`..` 越界、重复解码越界和符号链接逃逸。

### Markdown 与链接

- Markdown 语法固定为 Goldmark 标准 GFM，不提供额外扩展选项。
- raw HTML 和危险 URL 默认禁用，只有 `--unsafe-html` 显式启用 `html.WithUnsafe()`。
- 本地 Markdown 链接由一个 AST renderer 统一改写，并保留 query 与 fragment：

| 输入 | `convert` 输出 | `preview` 输出 |
| --- | --- | --- |
| `guide.md` | `guide.html` | `/doc/guide.md` |
| `./guide.md#start` | `./guide.html#start` | `/doc/guide.md#start` |
| `../guide.md` | `../guide.html` | 按当前文件解析后的 `/doc/.../guide.md` |
| `https://example.com/a.md` | 不修改 | 不修改 |
| `#anchor` | 不修改 | 不修改 |

- `convert` 保持本地图片和附件的相对 URL，并按原目录结构复制资源。
- `preview` 将本地附件 URL 改写到受根目录约束的 `/assets/<relative-path>`；`/doc/*` 专用于 SPA 文档路由和 fallback。
- 标题在 Go 端从 Goldmark AST 的第一个 H1 提取；没有 H1 时使用带扩展名的文件名。React 不解析返回 HTML 推导标题。

### 主题与页面布局

- HTML 使用固定版本的 `github-markdown-css`；升级时记录上游版本和许可证。
- 同一份 vendored CSS 由 `internal/assets` 嵌入并供 convert、单文件 preview、目录 WebUI 共用，避免前后端样式漂移。
- `.markdown-body` 最大宽度 `980px`、桌面 padding `45px`、宽度不超过 `767px` 时 padding `15px`。
- `auto` 使用 `prefers-color-scheme`，页面背景、正文与代码高亮必须一起切换，不能只切换正文颜色。

### preview 服务模式

- 单文件模式只返回文档正文页；监听父目录而非文件本身，以支持编辑器 atomic save；SSE 发送 `document-changed`。
- 目录模式不启用 fsnotify。刷新按钮重新请求 `/api/files`；之后打开文档时 `/api/document` 每次从磁盘读取最新内容。
- `/api/events` 保留 SSE 契约：单文件模式发送 `document-changed`；`tree-changed` 事件名保留给未来显式启用目录监听，不在默认目录模式产生。
- 浏览器只在服务成功 bind 后、且明确传入 `--browser` 时打开。
- SPA 文档路由是 `/doc/<relative-markdown-path>`，直接刷新任何该路由都返回嵌入的 `index.html`。

## 技术基线与依赖策略

### Go

- Go `1.26.x`，CI 使用 `actions/setup-go@v6` 的 `1.26.x` 并开启 `check-latest`。
- CLI：`github.com/urfave/cli/v3`。
- glob：`github.com/bmatcuk/doublestar/v4`。
- 文件监听：`github.com/fsnotify/fsnotify`。
- Markdown：`github.com/yuin/goldmark` v1 最新稳定版。
- 代码高亮：`github.com/yuin/goldmark-highlighting/v2`。
- 终端渲染：`github.com/charmbracelet/glamour` 最新稳定版，行为参考 Glow。

### Web

- React 19、TypeScript 7.x、Vite、Vitest、Biome。
- shadcn 使用 Base UI 组件源；基础组件限定为 Sidebar、ScrollArea、Button、Tooltip、Separator。
- 图标使用 `lucide-react`；Markdown 样式使用与 Go 端同版本的 `github-markdown-css`。

### 版本固定规则

- 阶段 1/5 开始时查询并安装满足上述大版本约束的最新稳定版，不使用 beta、rc 或 floating branch。
- Go 版本写入 `go.mod/go.sum`；Web 版本写入 `package.json/package-lock.json`，CI 只使用 lockfile 安装。
- 每次依赖升级单独提交，运行 `make check` 和 `make build-all`，并在 PR 中列出实际版本变化。

## 阶段看板

| 阶段 | 交付内容 | 依赖 | 状态 |
| --- | --- | --- | --- |
| 0 | 文档、目录、Makefile、CI、实施计划 | 无 | 已完成 |
| 1 | Go/Web 工具链、CLI 骨架、版本 | 阶段 0 | 已完成（v0.1.0） |
| 2 | 共享 Markdown 渲染核心 | 阶段 1 | 已完成（v0.2.0） |
| 3 | 文件发现与 `convert` | 阶段 2 | 已完成（v0.3.0） |
| 4 | 单文件 `preview`、watcher、SSE | 阶段 2 | 已完成（v0.4.0） |
| 5 | 目录 `preview` API 与安全边界 | 阶段 3、4 | 未开始 |
| 6 | React 目录 WebUI | 阶段 5 | 未开始 |
| 7 | `view` 终端预览 | 阶段 2 | 未开始 |
| 8 | 跨平台发布与文档收口 | 阶段 3–7 | 未开始 |

## 阶段 1：工具链、CLI 骨架与版本

### 交付物

- [x] 创建 `go.mod/go.sum`，module path 使用仓库的最终 GitHub 路径，声明 Go 1.26。
- [x] 创建 `main.go`，只负责构建 `cli.Command`、注入版本并返回进程退出码。
- [x] 在 `internal/cli` 注册 `version`、`convert`、`preview`、`view` 命令和完整 flags；未实现 handler 返回明确的开发期错误。
- [x] 在 `internal/version` 实现版本值、格式校验和 CLI 输出。
- [x] 初始化 React 19 + TypeScript 7 + Vite + Vitest + Biome 的最小可构建 Web 项目和 lockfile，不实现目录 UI。
- [x] 让 `make setup`、`make build`、`make test`、`make check` 可执行，并让 CI readiness 自动进入测试/构建分支。

### 测试与验收

- [x] `m2h version`、`m2h --version`、`make version` 输出一致。
- [x] 无 tag 构建为 `dev-<commit-date>-<commit7>`；精确 `v1.2.3` tag 构建为 `1.2.3`。
- [x] 所有未知 flag 返回 `Error: unknown option` 和非零退出码。
- [x] 命令帮助包含 README 中约定的参数、默认值和短选项。
- [x] `make check` 与 `make build` 通过；CI 至少完成一次 Linux test job。

## 阶段 2：共享 Markdown 渲染核心

### 交付物

- [x] 在 `internal/markdown` 定义 `RenderOptions`、主题、输出目标（convert/preview）和渲染结果（HTML、Title）。
- [x] 配置固定的 Goldmark GFM、代码高亮和 raw HTML 安全开关。
- [x] 实现 AST 级本地 Markdown link rewriter，不用渲染后字符串替换。
- [x] 实现 AST 级首个 H1 标题提取；标题纯文本化时正确处理行内代码、链接和强调。
- [x] 在 `internal/assets` 固定 `github-markdown-css`、代码主题 CSS、m2h layout CSS 和上游许可证。
- [x] 生成完整 HTML 文档模板，主题 `light/dark/auto` 覆盖页面背景、正文、代码块和移动端布局。

### 测试与验收

- [x] fixture 覆盖 GFM 表格、任务列表、删除线、自动链接、围栏代码和语言高亮。
- [x] raw HTML 默认被抑制，`unsafe-html=true` 才原样渲染；危险 URL 默认仍安全处理。
- [x] 链接矩阵覆盖相对路径、父目录、query、fragment、绝对 URL、mailto、anchor 和非 Markdown 后缀。
- [x] convert 与 preview 对同一输入产生相同正文 AST 输出，仅链接目标和外层页面不同。
- [x] CSS 快照包含 980px、45px/15px 和 light/dark/auto 规则。

## 阶段 3：文件发现与 `convert`

### 交付物

- [x] 在 `internal/convert` 区分文件/目录输入，统一规范化尾部 `/`、绝对路径和根目录符号链接。
- [x] 使用 doublestar 实现 depth + glob 过滤；结果按规范化相对路径稳定排序。
- [x] 单文件默认写入同目录同名 `.html`，`--output/-o` 可指定文件。
- [x] 目录输出保留相对结构；未指定 output 时 HTML 写在源文件旁边。
- [x] `--copy-assets=true` 默认复制非 Markdown 文件，false 时只生成 HTML。
- [x] output 位于 source 内时排除 output subtree，防止重复遍历；写入使用临时文件 + rename。
- [x] 所有目标冲突、权限错误和部分失败返回上下文错误，不能把失败批次报告为成功。

### 测试与验收

- [x] 测试深度 0/1/2/3、`*`/`**`、Windows 分隔符归一化、空匹配和非法 glob。
- [x] 测试目录有无尾部 `/` 输出完全一致。
- [x] 测试资源复制、目录结构、同名冲突、output-in-source 和 copy-assets=false。
- [x] 测试根输入 symlink、内部 symlink directory 跳过、symlink 越界保护。
- [x] `--glob`、`--depth` 用于单文件时返回针对性错误。
- [x] `make test` 和针对 convert fixture 的集成测试通过。

## 阶段 4：单文件 `preview`、watcher 与 SSE

### 交付物

- [x] 在 `internal/server` 实现 `--host`、`--port/-p`、优雅关闭、错误日志和单文档 HTML handler；默认监听 `127.0.0.1:8793`。
- [x] 在 `internal/watcher` 监听输入文件的父目录，按目标 basename 过滤 create/write/rename 事件并做 debounce。
- [x] 实现 `/api/events` SSE：连接保活、客户端断开清理、`document-changed` 广播。
- [x] 单文件页面不渲染标题栏、文件树或操作按钮，只渲染 `.markdown-body`。
- [x] `--browser` 在监听成功后打开平台默认浏览器；失败只记录错误，不终止已启动服务。
- [x] 为本地附件提供安全 `/assets/*` 路由，资源读取以 Markdown 父目录为 root。

### 测试与验收

- [x] 测试普通写入与 temp-file + rename 两种保存方式都触发一次有效刷新。
- [x] 测试无关文件变化不触发、事件 burst 被合并、客户端断开不泄漏 goroutine。
- [x] 测试默认 `127.0.0.1:8793`、自定义 host/port、端口占用与信号关闭。
- [x] 测试路径穿越、编码穿越和 symlink 越界附件请求均被拒绝。
- [ ] 浏览器手动验收 light/dark/auto、代码块、移动端 padding 和实时刷新（自动化 HTTP、SSE 与 CSS 契约已通过，保留给用户做浏览器验收）。

## 阶段 5：目录 `preview` API 与安全边界

### 交付物

- [ ] `GET /api/files` 返回按相对路径排序的 `{path,name,title}`，title 来自 Go AST。
- [ ] `GET /api/document?path=...` 每次读取磁盘并返回 `{path,title,html}`。
- [ ] `GET /assets/*` 返回根目录内非 Markdown 附件，设置正确 content type 和缓存策略。
- [ ] `/doc/*` 对浏览器导航 fallback 到嵌入的 SPA `index.html`，API 404 保持 JSON 错误，不被 SPA 接管。
- [ ] 默认选择严格遵循 `README.md`、`index.md`、首个排序文件、空状态的优先级。
- [ ] 请求日志包含 method、route、相对文档路径、status、耗时；不记录文件正文。
- [ ] 目录模式不创建 watcher，刷新文件树只重新扫描一次。

### 测试与验收

- [ ] API contract 测试覆盖正常、空目录、文件删除、标题变化、非法 query 和不存在文件。
- [ ] depth、glob、尾部 `/` 和 symlink 规则与 convert 共享 fixture，并得到一致文件集合。
- [ ] 路径穿越、URL 编码、NUL、绝对路径、大小写边界和 symlink escape 被拒绝。
- [ ] 直接请求嵌套路由 `/doc/design/architecture.md` 返回 SPA；刷新不回到默认文档。
- [ ] 手动刷新后文件树更新，随后打开文档读取最新磁盘内容。

## 阶段 6：React 目录 WebUI

### 交付物

- [ ] 初始化 shadcn Base UI 组件：Sidebar、ScrollArea、Button、Tooltip、Separator，并使用 lucide 图标。
- [ ] 构建桌面侧栏 + 文档区布局；侧栏文件树默认折叠，当前文件自动展开并滚动定位。
- [ ] 用浏览器 History API 同步 `/doc/*`，启动时从 URL 恢复文档，不维护第二套路由状态。
- [ ] 顶部标题和 `document.title` 直接使用 API title；不解析 `.markdown-body h1`。
- [ ] mode 始终写入 `?mode=light|dark|auto`，默认 URL 为 `?mode=auto`。
- [ ] 刷新按钮重新加载文件树并保留仍存在的当前选择；当前文件已删除时回退到默认优先级。
- [ ] 空目录、加载中、API 错误、文档删除和附件失败都有可访问的状态反馈。

### 测试与验收

- [ ] Vitest 覆盖默认选择、深链接恢复、前进/后退、树展开定位、刷新与删除回退。
- [ ] 组件可通过键盘操作，有可访问名称、焦点可见，不用仅颜色表达状态。
- [ ] `npm run lint`、`npm run test`、`npm run build` 和 `make check` 通过。
- [ ] 手动浏览器验收桌面/移动布局、长标题、深层树、大文件滚动、三种主题和页面标题。

## 阶段 7：`view` 终端预览

### 交付物

- [ ] 使用 Glamour 渲染单个本地 Markdown 文件，不启动 Web 服务。
- [ ] `--mode light|dark|auto` 映射到终端样式；auto 使用终端背景检测能力，无法检测时使用稳定默认值。
- [ ] 非文件、目录输入、读取失败和未知 mode 返回明确错误与非零退出码。
- [ ] 复用共享输入安全和标准 GFM 语法配置；不复制浏览器 HTML/CSS 渲染路径。

### 测试与验收

- [ ] golden tests 覆盖标题、列表、表格、代码块、链接和 Unicode。
- [ ] 颜色输出在非 TTY/`NO_COLOR` 场景遵守终端约定。
- [ ] 大文件渲染可取消，错误不输出半截成功提示。
- [ ] `make test` 与三大平台基础 smoke test 通过。

## 阶段 8：跨平台发布与文档收口

### 交付物

- [ ] 完成 Go `//go:embed` WebUI 和 CSS 资产，验证发布二进制不依赖工作目录文件。
- [ ] `make build-all` 生成 linux/darwin/windows × amd64/arm64 六个二进制。
- [ ] `make dist` 生成六个压缩包和 `checksums.txt`，包内含二进制、README、LICENSE。
- [ ] build workflow 使用 checkout@v6、setup-node@v6、setup-go@v6、codecov@v7，并上传六平台 artifacts。
- [ ] release workflow 只接受 `vX.Y.Z` tag，复用完整检查与六平台构建，并从 CHANGELOG 对应版本生成或更新 GitHub Release。
- [ ] build/release 均将二进制与 SHA-256 上传到 `/Shares/github/<owner>/<repo>/<os>/<arch>/<version>/bin{,.sha256}`。
- [ ] 配置 `CODECOV_TOKEN`、`PUSHOVER_TOKEN`、`PUSHOVER_USER`、`WEBDAV_SERVER`、`WEBDAV_USERNAME`、`WEBDAV_PASSWORD`；缺少 Pushover secret 时安全跳过通知。
- [ ] README 移除“尚未实现”提示并以实测输出更新所有示例。
- [ ] 发布时把实际新特性/修复从 `[未发布]` 移到 `x.y.z`，内容与 GitHub Release 完全一致。

### 测试与验收

- [ ] `make clean && make setup && make check && make dist` 在干净 checkout 通过。
- [ ] 校验六个 archive、文件名、Windows `.exe`、SHA-256 和解压后的 `m2h version`。
- [ ] 在 macOS、Linux、Windows 至少各运行 version、单文件 convert、单文件 preview、view smoke test。
- [ ] 正式 tag `vX.Y.Z` 的二进制只输出 `X.Y.Z`；非 tag 构建输出开发版本格式。
- [ ] build/release 的 test、Codecov、六个 matrix build、artifact、WebDAV、GitHub Release 和 Pushover 状态全部验证。

## 横向测试矩阵

| 维度 | 必测值 |
| --- | --- |
| 输入 | 单文件、目录、尾部 `/`、根 symlink、空目录、不存在路径 |
| 路径 | ASCII、空格、Unicode、深层目录、`..`、URL 编码、Windows 分隔符 |
| Markdown | 标准 GFM、raw HTML、危险 URL、代码高亮、本地链接、附件 |
| mode | light、dark、auto、非法值 |
| 文件变化 | write、atomic rename、删除、重建、无关文件 burst |
| 平台 | linux/darwin/windows × amd64/arm64 构建 |
| API | 正常、400、404、路径逃逸、文件并发删除、客户端取消 |

## 每阶段完成规则

1. 只实现该阶段列出的交付物；新范围先更新本文件。
2. 为新增契约先写失败测试，再实现到通过；fixture 尽量在 convert/preview 间共享。
3. 运行阶段列出的验证，并记录未运行的浏览器或跨平台手工项。
4. 检查 `git diff --check`、敏感信息和生成文件，不提交本地索引或构建产物。
5. 更新本看板与 README 的“当前可用能力”。
6. 使用中文 Conventional Commit，每个阶段一个逻辑提交，不跳过 hooks。

## 当前已知风险

- TypeScript 7.x 或 Go 1.26.x 的具体最新补丁可能变化：阶段开始时查询稳定版并由 lockfile 固定。
- Goldmark 高亮主题与 `github-markdown-css` 的 light/dark 色值可能冲突：阶段 2 用 CSS fixture 和浏览器手工验收锁定。
- atomic save 在不同编辑器上产生的 fsnotify 事件序列不同：阶段 4 同时测试 write、rename、remove + create。
- 目录很大时标题提取需要解析所有 Markdown：先保证稳定排序与正确性，再用 benchmark 决定是否需要有界并发或缓存。
- Windows 路径、驱动器号和大小写规则与 Unix 不同：路径安全逻辑必须使用平台测试，不能只做字符串前缀判断。
- output 位于 source 内可能产生递归输入：阶段 3 在枚举前解析并排除目标子树。
