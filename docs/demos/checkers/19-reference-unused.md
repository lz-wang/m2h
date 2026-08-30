---
title: 从未被使用的 reference 定义
description: 演示从未被链接使用的 reference 定义警告。
tags:
  - 文档检查
  - 引用链接
create_date: 2026-08-30
update_date: 2026-08-30
checker: reference.unused
---

# 从未被使用的 reference 定义

检查规则：`reference.unused`

两个定义中只有一个被正文使用：

[官网][site]

[site]: https://example.com
[unused]: https://example.com/orphan

预期表现：

- `m2h check docs/demos/checkers/19-reference-unused.md` 报告
  `21:1: warning [reference.unused]: reference definition "unused" is never used`
- 标签按渲染器同一归一化比较：`[文字][SITE]` 同样算使用 `[site]`

[返回检查规则演示索引](00-index.md)
