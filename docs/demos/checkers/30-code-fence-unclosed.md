---
title: 没有关闭的代码围栏
description: 演示 fenced code 缺少显式关闭行时的错误。
tags:
  - 文档检查
  - 代码块
create_date: 2026-11-14
update_date: 2026-11-14
checker: code-fence.unclosed
---

# 没有关闭的代码围栏

检查规则：`code-fence.unclosed`

CommonMark 允许代码围栏在文档末尾或容器结束时隐式终止，渲染器也照常
输出；但围栏一旦丢失关闭行，其后所有内容都会被吞进代码块。m2h 把
"缺少显式关闭"定义为文档完整性错误，而不是语法错误。本演示最后的
` ```text ` 围栏没有关闭行，因此报告本规则；预期说明放在围栏之前，
正是为了避免被它吞掉。

预期表现：

- `m2h check docs/demos/checkers/30-code-fence-unclosed.md` 报告
  `35:1: error [code-fence.unclosed]: fenced code block is not closed`
- 判定以 Goldmark AST 确认的代码块为准：关闭行必须与开围栏同字符、
  长度不短于开围栏、其后只剩空白，且没有越过所在容器（列表、引用）
  的边界；HTML 块与 Frontmatter 里的三个反引号不误报
- 缺语言与缺关闭是两个独立问题，可以同时报告
- 已被解析器隐式终止（文档末尾或容器结束）的围栏仍报告，因为本规则
  要求显式关闭

[返回检查规则演示索引](00-index.md)

```text
这一段代码没有结束围栏，文档到结尾都算它的内容。
