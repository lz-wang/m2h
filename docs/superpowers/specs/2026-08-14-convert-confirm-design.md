# 转换前确认提示与 `--yes` 跳过 — 设计文档

日期：2026-08-14
状态：已与用户逐节确认

## 背景与目标

`m2h <file|directory>` 的默认转换动作会写入文件：单文件在源文件旁生成 `.html`；目录模式默认就地写入源目录（生成 `.html`、复制资源、创建 `.m2h/` 运行时目录），重复执行会覆盖已有文件。目前执行前没有任何拦截。

目标：为默认转换动作增加执行前确认提示，并提供 `--yes`（`-y`）跳过。`m2h web` 只读预览，不受影响。

## 已确认的需求决策

1. **确认时机**：每次转换都确认，不做"仅覆盖时确认"的条件分支。
2. **非交互环境**：stdin 不是终端（脚本 / CI / 管道 / 重定向）时直接报错，要求加 `--yes` 重跑；加 `--yes` 则直接执行。
3. **默认回答**：提示为 `[y/N]`，直接回车或输入 `y`/`Y`/`yes` 之外的任意内容（含 EOF）均取消。

## 交互流程

```
$ m2h docs
Convert docs in place (may overwrite existing HTML)? [y/N] n
Aborted.

$ m2h README.md
Convert README.md to README.html? [y/N] y
Converted 1 Markdown file.
Output HTML files:
- /work/project/README.html
```

### 提示语形态（四种）

| 场景 | 提示语 |
| --- | --- |
| 单文件，无 `-o` | `Convert README.md to README.html? [y/N] `（目标按"去扩展名 + `.html`"推导，与 `convert.runFile` 同规则） |
| 单文件，有 `-o` | `Convert README.md to public/index.html? [y/N] ` |
| 目录，无 `-o` | `Convert docs in place (may overwrite existing HTML)? [y/N] ` |
| 目录，有 `-o` | `Convert docs into public/docs? [y/N] ` |

- 提示语中的路径按用户输入原样显示（`-o` 值、推导目标均不做绝对化）。
- 输入处理：读一行，忽略首尾空白与大小写；仅 `y` / `yes` 为确认。
- 取消：向 stderr 打印 `Aborted.`，退出码 0（用户主动取消不算失败，与 `rm -i` 惯例一致；脚本场景到不了这一步——非交互无 `--yes` 已先行报错）。
- 提示写入 **stderr**（与日志一致），stdout 保持纯净可管道。
- 提示文案用英文，与现有 CLI 输出（`Converted N Markdown files.`、`Error: ...`）一致。

## 参数与非交互行为

- 根命令新增布尔选项 `--yes`（别名 `-y`），usage：`skip the conversion confirmation prompt`，`Local: true`。`-y` 不与现有短选项冲突（`-v` 被 version 占用）。
- 确认只发生在默认转换动作内、**参数校验之后**（0 参数显示帮助、多参数报错，不提示）；`m2h web`、`--version` 不涉及。
- TTY 判定：注入的 stdin 是 `*os.File` 且 `term.IsTerminal(fd)` 为真才算交互环境。
- 非交互且未加 `--yes`：

  ```
  $ m2h docs < /dev/null
  Error: conversion requires confirmation; rerun with --yes
  ```

  退出码 1（标准错误路径）。

## 代码结构（方案 A：确认全部收在 `internal/cli` 层）

`internal/convert` 保持纯转换逻辑，不感知 stdin。

### 新增 `internal/cli/confirm.go`

- `interactiveStdin(reader io.Reader) bool` — reader 是 `*os.File` 且为终端才算交互。TTY 判定经包内可替换钩子 `stdinIsTerminal`（生产实现 `term.IsTerminal(fd)`，测试替换为桩），保证交互路径可测。
- `confirm(reader io.Reader, writer io.Writer, prompt string) bool` — 向 writer 写提示，`bufio` 读一行，按上述规则判定。
- `convertPrompt(input, output string) string` — 生成四种提示语；用一次 `os.Stat` 区分文件 / 目录。
  - **边界**：输入 stat 失败（不存在等）时跳过确认直接执行，交给 convert 报它自己的标准错误——输入不存在时没有"确认覆盖"的前提。

### 修改 `internal/cli/cli.go`

- `New()` 签名新增 `stdin io.Reader` 参数。
- `conversionFlags()` 追加 `--yes` / `-y`。
- `convertAction` 在参数校验后、`runConvert` 前：未加 `--yes` 时，非交互报错，交互则提示确认。

### 修改 `main.go` / `main_test.go`

`run` / `runContext` 透传 stdin，`main` 传 `os.Stdin`。

### 依赖

新增 `golang.org/x/term`（仅用 `IsTerminal`；`golang.org/x/sys` 已在依赖树中，实际增量极小）。

## 测试

`internal/cli/confirm_test.go` 新增 + `cli_test.go` 增补：

- 四种提示语文案断言。
- `y` / `Y` / `yes` / ` y `（带空白）均确认；`n` / 直接回车 / EOF 均取消。
- 取消后打印 `Aborted.`、**不产生任何输出文件**、退出码 0。
- 注入 `bytes.Reader`（非 TTY）无 `--yes` → 报错退出 1；加 `--yes` → 无提示直接转换。
- 桩掉 `stdinIsTerminal` 后覆盖交互确认 → 执行的完整链路（文件真实生成）。
- 既有 web / help / `--version` 用例回归不受影响。

覆盖率维持后端不低于 90% 的仓库约定。

## 文档同步

- `README.md`："转换为 HTML"小节说明转换前会提示确认及跳过方法；选项表新增 `--yes`, `-y` 行。
- `CHANGELOG.md` `[未发布]` 新增条目。
- `go mod tidy` 同步 `go.sum`。

## 已知限制

- 提示等待输入期间按 `Ctrl+C`，进程需等到下一次回车才退出（Go 阻塞读 stdin 的通病）；与大量 Go CLI 行为一致，不做特殊处理。
