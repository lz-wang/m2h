---
title: 没有正文的章节
description: 演示没有任何渲染内容的章节警告。
tags:
  - 文档检查
  - 标题结构
create_date: 2026-08-30
update_date: 2026-08-30
checker: section.empty
---

检查规则：`section.empty`

# 标题

## 空章节

## 下一节

"空章节"和下一节之间没有任何渲染内容，本节自身也没有正文。

预期表现：

- 默认不检查：`m2h check docs/demos/checkers/22-section-empty.md` 无诊断
- `m2h check docs/demos/checkers/22-section-empty.md --enable section.empty`
  报告 `14:1` 与 `16:1` 两条 `warning [section.empty]: section … has no content`
  （H1 只承载子标题同样算空）
- 父章节只做结构、正文全在子章节是常见合法写法，因此本规则默认关闭
- thematic break（`---`/`***`）会渲染为 `<hr>`，因此计为内容；reference
  definition、HTML comment 与空白不计为渲染内容

[返回检查规则演示索引](00-index.md)
