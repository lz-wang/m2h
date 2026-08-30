---
title: 从未被引用的脚注
description: 演示定义了内容但从未被正文引用的脚注警告。
tags:
  - 文档检查
  - 脚注
create_date: 2026-08-30
update_date: 2026-08-30
checker: footnote.unused
---

# 从未被引用的脚注

检查规则：`footnote.unused`

下面的脚注有内容，但正文从头到尾没有出现过 `[^orphan]` 标记，读者永远走
不到这里：

[^orphan]: 一条有内容却从未被引用的脚注。

预期表现：

- `m2h check docs/demos/checkers/18-footnote-unused.md` 报告
  `19:1: warning [footnote.unused]: footnote [^orphan] is never referenced`
- 定义有内容，因此不触发 `footnote.empty`

[返回检查规则演示索引](00-index.md)
