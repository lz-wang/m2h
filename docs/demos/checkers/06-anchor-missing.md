---
title: 锚点不存在
---

# 锚点不存在

本文档没有任何 ID 为 `missing-anchor` 的标题，下面这个页内锚点永远跳不到
目的地：

[不存在的锚点](#missing-anchor)

预期表现：

- `m2h check docs/demos/checkers/06-anchor-missing.md` 报告
  `10:2: error [anchor.missing]: heading "#missing-anchor" does not exist in "06-anchor-missing.md"`
- 锚点判定与 WebUI 目录使用同一 GitHub 兼容 ID 算法，跨文档锚点
  （`target.md#anchor`）走同一规则
