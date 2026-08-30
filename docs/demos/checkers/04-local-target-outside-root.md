---
title: 越出文档根目录
---

# 越出文档根目录

下面这个链接用连续的 `../` 走出了被检查的根目录，无论目标是否真实存在，
工作区都不会提供它：

[越出根目录的引用](../../../../etc/hosts)

预期表现：

- `m2h check docs/demos/checkers/04-local-target-outside-root.md` 报告
  `10:2: error [local-target.outside-root]: target "../../../../etc/hosts" resolves outside the workspace root`
