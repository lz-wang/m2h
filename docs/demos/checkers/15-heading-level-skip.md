---
title: 标题层级向下跳跃
description: 演示标题层级向下跨越多级时的警告。
tags:
  - 文档检查
  - 标题结构
create_date: 2026-08-30
update_date: 2026-08-30
checker: heading.level-skip
---

# API 参考

检查规则：`heading.level-skip`

下面直接从 H1 跳到 H4，目录树里中间凭空缺了两层：

#### 参数列表

预期表现：

- `m2h check docs/demos/checkers/15-heading-level-skip.md` 报告
  `18:1: warning [heading.level-skip]: heading level jumps from H1 to H4`
- 向上跳任意级是合法的章节收尾；文档第一条标题即使是 H3 也不报

[返回检查规则演示索引](00-index.md)
