---
title: 标签数量不在范围内
description: 演示 frontmatter.tags-count 的判定与边界。
tags: []
create_date: 2026-09-10
update_date: 2026-09-10
---

# 标签数量不在范围内

检查规则：`frontmatter.tags-count`

仅在 tags 字段存在时检查，缺少 Frontmatter 或 tags 均不告警。
复用展示层规范化：标量算一个标签（不按逗号拆分），序列去除空白值、null、重复项与非标量项。
规范化后允许 1–5 个标签；空序列、空值或映射会因零个有效标签而告警。
本例 tags 故意为空，诊断定位到 tags 字段。

`4:1: warning [frontmatter.tags-count]: frontmatter has 0 tags; expected 1 to 5`

[返回检查规则演示索引](00-index.md)
