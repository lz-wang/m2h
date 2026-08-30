---
title: 代码块没有语言
description: 演示 fenced code 未标注语言时的警告。
tags:
  - 文档检查
  - 代码块
create_date: 2026-08-30
update_date: 2026-08-30
checker: code-fence.language-missing
---

# 代码块没有语言

检查规则：`code-fence.language-missing`

下面的 fenced code 没有写语言，渲染器无法做语法高亮：

```
func main() { println("hello") }
```

预期表现：

- `m2h check docs/demos/checkers/17-code-fence-language-missing.md` 报告
  `18:1: warning [code-fence.language-missing]: fenced code block has no language`
- 只检查 fenced code；缩进四个空位的 indented code 不适用本规则
- blockquote 与 `-`/`*`/`+` 列表项中的 fence 同样由 AST 识别；即使 fence
  没有内容或 info string，诊断也定位到 opener 行

[返回检查规则演示索引](00-index.md)
