# 转换前确认提示与 --yes 跳过 实现计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** `m2h <file|directory>` 默认转换动作在写入前显示终端确认提示（`[y/N]`，默认取消），新增 `--yes`/`-y` 跳过；非终端 stdin 不加 `--yes` 时报错。

**Architecture:** 确认交互全部收在 `internal/cli` 层（新文件 `confirm.go`），`internal/convert` 保持纯转换逻辑不感知 stdin。TTY 检测用 `golang.org/x/term`，经包内可替换变量 `interactiveStdin` 暴露给测试打桩。

**Tech Stack:** Go 1.26、urfave/cli v3、golang.org/x/term（新增）、标准 testing。

**规格文档:** `docs/superpowers/specs/2026-08-14-convert-confirm-design.md`

## Global Constraints

- Go 代码使用 Tab 缩进（gofmt / goimports-reviser，导入顺序 std,general,company,project,blanked,dotted）。
- 提交信息使用中文 Conventional Commits，一个提交只含一个逻辑变更；提交信息结尾加 `Co-Authored-By: Claude <noreply@anthropic.com>`。
- CLI 用户可见文案用英文；错误消息以 `Error: ` 开头（与现有风格一致）。
- 非交互错误消息逐字为：`Error: conversion requires confirmation; rerun with --yes`。
- 取消提示逐字为：`Aborted.`（写 stderr，带换行）；确认提示以 `[y/N] ` 结尾、无换行。
- 后端测试覆盖率不得低于 90%。
- 面向用户的新特性必须在同一变更中写入 `CHANGELOG.md` 的 `[未发布]` 并同步 `README.md`。
- 不修改 `internal/convert` 与 `internal/server` 的任何文件。

---

### Task 1: `confirm` 读取判定函数

**Files:**
- Create: `internal/cli/confirm.go`
- Test: `internal/cli/confirm_test.go`

**Interfaces:**
- Consumes: 无
- Produces: `func confirm(reader io.Reader, writer io.Writer, prompt string) bool`（Task 3 使用）

- [ ] **Step 1: 写失败测试**

创建 `internal/cli/confirm_test.go`（注意 Go 代码用 Tab 缩进）：

```go
package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestConfirmAcceptsExplicitYesOnly(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{name: "lowercase y", input: "y\n", want: true},
		{name: "uppercase y", input: "Y\n", want: true},
		{name: "yes", input: "yes\n", want: true},
		{name: "YES uppercase", input: "YES\n", want: true},
		{name: "padded y", input: "  y  \n", want: true},
		{name: "y without trailing newline", input: "y", want: true},
		{name: "n declines", input: "n\n", want: false},
		{name: "empty line declines", input: "\n", want: false},
		{name: "eof declines", input: "", want: false},
		{name: "garbage declines", input: "maybe\n", want: false},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			var prompt bytes.Buffer
			got := confirm(strings.NewReader(test.input), &prompt, "Proceed? [y/N] ")
			if got != test.want {
				t.Fatalf("confirm(%q) = %v, want %v", test.input, got, test.want)
			}
			if got := prompt.String(); got != "Proceed? [y/N] " {
				t.Fatalf("prompt output = %q, want prompt echoed verbatim", got)
			}
		})
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./internal/cli/ -run TestConfirmAcceptsExplicitYesOnly -v`
Expected: 编译失败 `undefined: confirm`

- [ ] **Step 3: 最小实现**

创建 `internal/cli/confirm.go`：

```go
package cli

import (
	"bufio"
	"fmt"
	"io"
	"strings"
)

// confirm prints prompt to writer and reads one line from reader. It reports
// true only for an explicit y/yes answer (case-insensitive, surrounding
// whitespace ignored); an empty answer or read failure declines.
func confirm(reader io.Reader, writer io.Writer, prompt string) bool {
	if _, err := fmt.Fprint(writer, prompt); err != nil {
		return false
	}
	line, err := bufio.NewReader(reader).ReadString('\n')
	if err != nil && line == "" {
		return false
	}
	switch strings.ToLower(strings.TrimSpace(line)) {
	case "y", "yes":
		return true
	default:
		return false
	}
}
```

- [ ] **Step 4: 运行测试确认通过**

Run: `go test ./internal/cli/ -run TestConfirmAcceptsExplicitYesOnly -v`
Expected: PASS

- [ ] **Step 5: 提交**

```bash
git add internal/cli/confirm.go internal/cli/confirm_test.go
git commit -m "feat(cli): 新增确认提示读取判定函数

Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

### Task 2: 终端检测钩子与确认提示语

**Files:**
- Modify: `internal/cli/confirm.go`（追加两个函数）
- Modify: `internal/cli/confirm_test.go`（追加测试）
- Modify: `go.mod` / `go.sum`（新增 `golang.org/x/term`）

**Interfaces:**
- Consumes: 无
- Produces:
  - `var interactiveStdin func(reader io.Reader) bool`（Task 3 调用、测试打桩）
  - `func convertPrompt(input, output string) string`（Task 3 调用；输入不可 stat 时返回 `""` 表示跳过确认）

- [ ] **Step 1: 添加依赖**

Run: `go get golang.org/x/term && go mod tidy`
Expected: go.mod 的 require 块出现 `golang.org/x/term`，命令成功退出

- [ ] **Step 2: 写失败测试**

在 `internal/cli/confirm_test.go` 追加（import 块补 `"fmt"`、`"os"`、`"path/filepath"`）：

```go
func TestInteractiveStdinRejectsNonFileReaders(t *testing.T) {
	t.Parallel()

	if interactiveStdin(strings.NewReader("y\n")) {
		t.Fatal("interactiveStdin(strings.Reader) = true, want false for non-file readers")
	}
}

func TestConvertPromptDescribesWriteTarget(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	source := filepath.Join(root, "guide.md")
	if err := os.WriteFile(source, []byte("# Guide"), 0o644); err != nil {
		t.Fatal(err)
	}
	directory := t.TempDir()

	tests := []struct {
		name   string
		input  string
		output string
		want   string
	}{
		{
			name:   "file without output derives html target",
			input:  source,
			output: "",
			want:   fmt.Sprintf("Convert %s to %s? [y/N] ", source, filepath.Join(root, "guide.html")),
		},
		{
			name:   "file with output echoes it",
			input:  source,
			output: "public/index.html",
			want:   "Convert " + source + " to public/index.html? [y/N] ",
		},
		{
			name:   "directory without output warns in place",
			input:  directory,
			output: "",
			want:   "Convert " + directory + " in place (may overwrite existing HTML)? [y/N] ",
		},
		{
			name:   "directory with output echoes it",
			input:  directory,
			output: "public/docs",
			want:   "Convert " + directory + " into public/docs? [y/N] ",
		},
		{
			name:   "missing input returns empty prompt",
			input:  filepath.Join(root, "missing.md"),
			output: "",
			want:   "",
		},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			if got := convertPrompt(test.input, test.output); got != test.want {
				t.Fatalf("convertPrompt(%q, %q) = %q, want %q", test.input, test.output, got, test.want)
			}
		})
	}
}
```

- [ ] **Step 3: 运行测试确认失败**

Run: `go test ./internal/cli/ -run 'TestInteractiveStdin|TestConvertPrompt' -v`
Expected: 编译失败 `undefined: interactiveStdin`、`undefined: convertPrompt`

- [ ] **Step 4: 实现**

在 `internal/cli/confirm.go` 追加（import 块补 `"os"`、`"path/filepath"` 与第三段 `golang.org/x/term`，保持 std 在前）：

```go
// interactiveStdin reports whether reader is an interactive terminal. It is a
// package variable so tests can force the interactive path without a TTY.
var interactiveStdin = func(reader io.Reader) bool {
	file, ok := reader.(*os.File)
	return ok && term.IsTerminal(int(file.Fd()))
}

// convertPrompt renders the confirmation prompt shown before a conversion.
// Paths are echoed exactly as the user typed them. A missing input returns an
// empty prompt: nothing exists to overwrite yet, so conversion should run and
// report its own error.
func convertPrompt(input, output string) string {
	info, err := os.Stat(input)
	if err != nil {
		return ""
	}
	if info.IsDir() {
		if output == "" {
			return fmt.Sprintf("Convert %s in place (may overwrite existing HTML)? [y/N] ", input)
		}
		return fmt.Sprintf("Convert %s into %s? [y/N] ", input, output)
	}
	target := output
	if target == "" {
		target = strings.TrimSuffix(input, filepath.Ext(input)) + ".html"
	}
	return fmt.Sprintf("Convert %s to %s? [y/N] ", input, target)
}
```

- [ ] **Step 5: 运行测试确认通过**

Run: `go test ./internal/cli/ -v`
Expected: 全部 PASS（含 Task 1 的用例）

- [ ] **Step 6: 提交**

```bash
git add internal/cli/confirm.go internal/cli/confirm_test.go go.mod go.sum
git commit -m "feat(cli): 新增终端检测钩子与转换确认提示语

Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

### Task 3: 接线到转换命令（含 README / CHANGELOG 同步）

**Files:**
- Modify: `internal/cli/cli.go`（`New` 签名、`conversionFlags`、`convertAction`、新增 `confirmConversion`）
- Modify: `internal/cli/cli_test.go`（helper 拆分、既有用例更新、新增行为用例）
- Modify: `main.go`（透传 stdin）
- Modify: `main_test.go`（`run` 签名）
- Modify: `README.md`（"转换为 HTML"小节与选项表）
- Modify: `CHANGELOG.md`（`[未发布]` 新增条目）

**Interfaces:**
- Consumes: `confirm(reader, writer, prompt) bool`、`interactiveStdin` 变量、`convertPrompt(input, output) string`（Task 1/2 产出）
- Produces: `New(buildVersion string, ui fs.FS, stdin io.Reader, stdout, stderr io.Writer) (*urfavecli.Command, error)`（main.go 使用）

- [ ] **Step 1: 写失败的行为测试**

在 `internal/cli/cli_test.go`：

1. 将现有 helper `runCommand`（cli_test.go:17-29）替换为以下两个函数（`import` 块已有 `strings`）：

```go
func runCommand(t *testing.T, args ...string) (string, string, error) {
	t.Helper()
	return runCommandInput(t, strings.NewReader(""), args...)
}

func runCommandInput(t *testing.T, stdin io.Reader, args ...string) (string, string, error) {
	t.Helper()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	command, err := New("dev-20260809-fe65804", testUI(), stdin, &stdout, &stderr)
	if err != nil {
		t.Fatalf("New() returned error: %v", err)
	}

	err = command.Run(context.Background(), append([]string{"m2h"}, args...))
	return stdout.String(), stderr.String(), err
}
```

（import 块补 `"io"`。）

2. 追加三个新测试函数：

```go
func TestConvertRequiresYesWithoutInteractiveStdin(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	source := filepath.Join(root, "guide.md")
	if err := os.WriteFile(source, []byte("# Guide"), 0o644); err != nil {
		t.Fatal(err)
	}

	stdout, stderr, err := runCommand(t, source)
	if err == nil || err.Error() != "Error: conversion requires confirmation; rerun with --yes" {
		t.Fatalf("convert error = %v, want confirmation requirement error", err)
	}
	if stdout != "" || stderr != "" {
		t.Fatalf("convert wrote stdout=%q stderr=%q, want none", stdout, stderr)
	}
	if _, err := os.Stat(filepath.Join(root, "guide.html")); !os.IsNotExist(err) {
		t.Fatal("conversion wrote HTML without confirmation")
	}
}

func TestConvertYesRunsConversionDirectly(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	source := filepath.Join(root, "guide.md")
	if err := os.WriteFile(source, []byte("# Guide"), 0o644); err != nil {
		t.Fatal(err)
	}

	stdout, stderr, err := runCommand(t, source, "--yes")
	if err != nil {
		t.Fatalf("convert --yes returned error: %v", err)
	}
	if stderr != "" || !strings.Contains(stdout, "Converted 1 Markdown file.") {
		t.Fatalf("convert --yes stdout=%q stderr=%q", stdout, stderr)
	}
	if _, err := os.Stat(filepath.Join(root, "guide.html")); err != nil {
		t.Fatalf("guide.html missing after --yes conversion: %v", err)
	}
}

func TestConvertInteractiveAnswerControlsExecution(t *testing.T) {
	previous := interactiveStdin
	interactiveStdin = func(io.Reader) bool { return true }
	t.Cleanup(func() { interactiveStdin = previous })

	// 注意：本用例及其子用例不能 t.Parallel()——它覆写了包级变量 interactiveStdin。
	newSource := func(t *testing.T) string {
		t.Helper()
		source := filepath.Join(t.TempDir(), "guide.md")
		if err := os.WriteFile(source, []byte("# Guide"), 0o644); err != nil {
			t.Fatal(err)
		}
		return source
	}

	tests := []struct {
		name       string
		answer     string
		wantRun    bool
		wantStderr string
	}{
		{name: "y proceeds", answer: "y\n", wantRun: true},
		{name: "Y proceeds", answer: "Y\n", wantRun: true},
		{name: "yes proceeds", answer: "yes\n", wantRun: true},
		{name: "padded y proceeds", answer: " y \n", wantRun: true},
		{name: "n aborts", answer: "n\n", wantStderr: "Aborted.\n"},
		{name: "enter aborts", answer: "\n", wantStderr: "Aborted.\n"},
		{name: "eof aborts", answer: "", wantStderr: "Aborted.\n"},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			source := newSource(t)
			target := strings.TrimSuffix(source, filepath.Ext(source)) + ".html"
			stdout, stderr, err := runCommandInput(t, strings.NewReader(test.answer), source)
			if err != nil {
				t.Fatalf("convert returned error: %v", err)
			}
			if want := "Convert " + source + " to " + target + "? [y/N] " + test.wantStderr; stderr != want {
				t.Fatalf("stderr = %q, want %q", stderr, want)
			}
			_, statErr := os.Stat(target)
			if test.wantRun && statErr != nil {
				t.Fatalf("HTML missing after confirmed conversion: %v", statErr)
			}
			if !test.wantRun && statErr == nil {
				t.Fatal("conversion ran after a declined answer")
			}
			if !test.wantRun && stdout != "" {
				t.Fatalf("declined conversion wrote stdout %q", stdout)
			}
		})
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./internal/cli/ -run 'TestConvert(RequiresYes|YesRuns|InteractiveAnswer)' -v`
Expected: 编译失败（`New` 参数数量不符）——先继续 Step 3 改签名后再看到断言失败

- [ ] **Step 3: 修改 `internal/cli/cli.go`**

1. `New` 签名（cli.go:30）改为并透传 stdin 到 Action：

```go
// New constructs the root command after validating the injected build version.
func New(buildVersion string, ui fs.FS, stdin io.Reader, stdout, stderr io.Writer) (*urfavecli.Command, error) {
```

Action 闭包内 `return convertAction(ctx, current)` 改为 `return convertAction(ctx, current, stdin)`。

2. `conversionFlags()`（cli.go:69-89）在 `--copy-assets` 之后追加：

```go
		&urfavecli.BoolFlag{
			Name:    "yes",
			Aliases: []string{"y"},
			Usage:   "skip the conversion confirmation prompt",
			Local:   true,
		},
```

3. `convertAction`（cli.go:93）签名改为 `func convertAction(ctx context.Context, command *urfavecli.Command, stdin io.Reader) error`，在参数数量校验之后、`runConvert` 之前插入：

```go
	proceed, err := confirmConversion(command, stdin)
	if err != nil {
		return err
	}
	if !proceed {
		return nil
	}
```

4. 在 `convertAction` 后新增：

```go
// confirmConversion asks the user to confirm the conversion unless --yes was
// given. Non-interactive stdin without --yes is an error; a declined answer
// prints "Aborted." and stops the command without failing it.
func confirmConversion(command *urfavecli.Command, stdin io.Reader) (bool, error) {
	if command.Bool("yes") {
		return true, nil
	}
	prompt := convertPrompt(command.Args().First(), command.String("output"))
	if prompt == "" {
		return true, nil
	}
	if !interactiveStdin(stdin) {
		return false, fmt.Errorf("Error: conversion requires confirmation; rerun with --yes")
	}
	if !confirm(stdin, command.Root().ErrWriter, prompt) {
		_, _ = fmt.Fprintln(command.Root().ErrWriter, "Aborted.")
		return false, nil
	}
	return true, nil
}
```

- [ ] **Step 4: 更新 `main.go` 与 `main_test.go`**

`main.go`：`main`、`run`、`runContext` 三处签名与调用改为（import 块已有 `"io"`）：

```go
func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	os.Exit(runContext(ctx, os.Args, webui.Content(), os.Stdin, os.Stdout, os.Stderr))
}

func run(args []string, ui fs.FS, stdin io.Reader, stdout, stderr io.Writer) int {
	return runContext(context.Background(), args, ui, stdin, stdout, stderr)
}

func runContext(ctx context.Context, args []string, ui fs.FS, stdin io.Reader, stdout, stderr io.Writer) int {
	command, err := appcli.New(M2HVersion, ui, stdin, stdout, stderr)
	if err == nil {
		err = command.Run(ctx, args)
	}
	if err != nil {
		_, _ = fmt.Fprintln(stderr, err)
		return 1
	}
	return 0
}
```

`main_test.go`：两处 `run(...)` 调用插入 stdin 参数（import 块补 `"strings"`）：

```go
			var stdin strings.Reader
			if got := run(test.args, webui.Content(), &stdin, &stdout, &stderr); got != test.wantCode {
```

```go
	var stdin strings.Reader
	if got := run([]string{"m2h", "--version"}, nil, &stdin, &stdout, &stderr); got != 1 {
```

- [ ] **Step 5: 更新既有用例**

`internal/cli/cli_test.go`：

1. `TestConvertCommandWritesHTML`（cli_test.go:281）参数加 `"--yes"`：

```go
	stdout, stderr, err := runCommand(t, source, "--yes", "--output", output, "--mode", "dark", "--width", "wide")
```

2. `TestConvertCommandWritesDirectoryResult`（cli_test.go:313）：

```go
	stdout, stderr, err := runCommand(t, source, "--yes", "--output", output)
```

3. `TestConvertCommandValidatesArgumentsAndDirectoryOnlyFlags`（cli_test.go:334-343）的 5 个用例全部加 `"--yes"`（它们要穿透确认直达 convert 内部校验）：

```go
		{args: []string{source, source}, want: "Error: requires exactly one file or directory"},
		{args: []string{source, "--yes", "--glob", "*.md"}, want: "Error: --glob can only be used when converting a directory"},
		{args: []string{source, "--yes", "--depth", "2"}, want: "Error: --depth can only be used when converting a directory"},
		{args: []string{source, "--yes", "--copy-assets=false"}, want: "Error: --copy-assets can only be used when converting a directory"},
		{args: []string{source, "--yes", "--standalone"}, want: "Error: --standalone can only be used when converting a directory"},
```

（第一条双参数用例不加 `--yes`：它在确认之前就被参数数量校验拒绝。`TestConvertCommandValidatesGlobBeforeInput` 的输入不存在，`convertPrompt` 返回空跳过确认，无需改动。）

4. `TestHelpDocumentsContract` root 分支的 `want` 列表（cli_test.go:85-86）补 `"--yes", "-y"`：

```go
				"--standalone", "--copy-assets", "(default: true)", "--yes", "-y", "--version", "-v",
```

5. `TestFlagsAreIsolatedBetweenCommands`（cli_test.go:143-152）的参数列表追加：

```go
		{"web", "README.md", "--yes"},
```

6. `TestInvalidVersionFailsConstruction`（cli_test.go:409）：

```go
	if _, err := New("invalid", nil, strings.NewReader(""), &bytes.Buffer{}, &bytes.Buffer{}); err == nil {
```

- [ ] **Step 6: 运行全部 CLI 与 main 测试确认通过**

Run: `go test ./internal/cli/ ./ -v`
Expected: 全部 PASS（含既有 web / help / version 用例）

- [ ] **Step 7: 手工冒烟（可选但推荐）**

Run（在仓库根目录执行）：

```bash
make build && mkdir -p /tmp/m2h-smoke && printf '# Hi\n' > /tmp/m2h-smoke/a.md
cd /tmp/m2h-smoke && /Users/lzwang/projects/m2h/m2h a.md --yes && ls a.html
/Users/lzwang/projects/m2h/m2h a.md < /dev/null; echo "exit=$?"
rm -rf /tmp/m2h-smoke && cd /Users/lzwang/projects/m2h
```

Expected: `a.html` 生成；`--yes` 无任何提示直接转换；`< /dev/null` 那行输出 `Error: conversion requires confirmation; rerun with --yes` 且 `exit=1`。

- [ ] **Step 8: 同步 README 与 CHANGELOG**

`README.md`：在"转换成功后，m2h 会向标准输出打印……"段落之前插入新段落：

```markdown
执行转换前，`m2h` 会在终端显示写入目标并要求确认（`[y/N]`，直接回车取消，仅 `y`/`yes` 继续）；加 `--yes` 或 `-y` 跳过确认。标准输入不是终端时（脚本、CI、管道）不加 `--yes` 会直接报错退出。`m2h web` 预览不受影响。
```

"转换为 HTML"选项表 `--width` 行之后追加一行：

```markdown
| `--yes`, `-y` | 跳过转换前的确认提示；非交互环境（脚本、CI、管道）必须加此选项。 |
```

`CHANGELOG.md` `[未发布]` 的 `### 新增` 列表末尾（"Web 预览刷新后恢复精确阅读位置"条目之后）追加：

```markdown
- 默认转换动作在写入前新增终端确认：显示输入与写入目标（单文件显示推导的 `.html` 路径，目录无 `--output` 时提示就地写入可能覆盖已有文件），`[y/N]` 直接回车取消、仅 `y`/`yes` 继续；新增 `--yes`（`-y`）选项跳过确认，标准输入不是终端时（脚本、CI、管道）不加 `--yes` 直接报错退出。`m2h web` 预览不受影响。
```

- [ ] **Step 9: 格式化并提交**

Run: `make format`
Expected: 无 diff 输出（或仅格式化本任务文件）

```bash
git add internal/cli/cli.go internal/cli/cli_test.go main.go main_test.go README.md CHANGELOG.md
git commit -m "feat(cli): 转换前要求终端确认并支持 --yes 跳过

Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

### Task 4: 全量验证

**Files:**
- 无代码改动（本任务只运行验证；若失败，修复后回到对应任务补提交）

**Interfaces:**
- Consumes: Task 1-3 的全部产出
- Produces: 无

- [ ] **Step 1: 静态检查与全部测试**

Run: `make check`
Expected: goimports-reviser 无 diff、`go mod tidy -diff` 无输出、`go vet` 通过、web-lint 与 web-test 通过（未改 Web 代码，应不受影响）、Go 测试全部 PASS

- [ ] **Step 2: 覆盖率不低于 90%**

Run: `make coverage && go tool cover -func=coverage.out | tail -1`
Expected: 末行 `total: (statements) 9x.x%`，数值 ≥ 90.0%

- [ ] **Step 3: 构建成功**

Run: `make build`
Expected: 输出 `[m2h] build ... -> ./m2h`，二进制生成成功

- [ ] **Step 4: 清理构建产物（不提交）**

Run: `rm -f ./m2h coverage.out coverage.html`
Expected: 工作区回到干净状态（`git status` 仅剩本计划文档如尚未提交）

---

## Self-Review 记录

- **Spec coverage**：确认时机/默认取消/非交互报错（Task 3 测试）、四种提示语与 stat 失败跳过（Task 2）、y/yes/EOF 判定（Task 1、Task 3）、Aborted. 退出码 0（Task 3）、`--yes` flag 与隔离（Task 3）、README/CHANGELOG 同步（Task 3 Step 8）、覆盖率 ≥90%（Task 4）——全覆盖。
- **Placeholder scan**：无 TBD/TODO/“适当处理”；所有代码步骤给出完整代码。
- **Type consistency**：`confirm`/`interactiveStdin`/`convertPrompt`/`New` 签名在 Task 1/2 定义与 Task 3 使用一致；`runCommandInput` 在 Task 3 定义并使用。
