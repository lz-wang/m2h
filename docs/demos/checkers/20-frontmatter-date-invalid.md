---
title: 日期字段不是有效 ISO 日期
description: 演示 Frontmatter 日期字段不是有效 ISO 日期时的警告。
tags:
  - 文档检查
  - frontmatter
create_date: 2026-08-30
update_date: 2026-08-30
date: 2026-02-30
checker: frontmatter.date-invalid
---

# 日期字段不是有效 ISO 日期

检查规则：`frontmatter.date-invalid`

上面的 frontmatter `date` 写了 2 月 30 日——这不是有效日期，永远不会进入
工具栏摘要。

预期表现：

- `m2h check docs/demos/checkers/20-frontmatter-date-invalid.md` 报告
  `9:1: warning [frontmatter.date-invalid]: date is not a valid ISO date`
- 诊断定位到该字段在 Frontmatter 中的真实行列；自由文本（如 `March 8th`）
  同样触发

[返回检查规则演示索引](00-index.md)
