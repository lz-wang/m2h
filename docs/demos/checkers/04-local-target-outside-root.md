---
title: 越出文档根目录
description: 演示检查器如何阻止本地链接越过文档根目录。
tags:
  - 文档检查
  - 路径安全
create_date: 2026-08-30
update_date: 2026-08-30
checker: local-target.outside-root
---

# 越出文档根目录

检查规则：`local-target.outside-root`

下面这个链接用连续的 `../` 走出了被检查的根目录，无论目标是否真实存在，
工作区都不会提供它：

[越出根目录的引用](../../../../etc/hosts)

预期表现：

- `m2h check docs/demos/checkers/04-local-target-outside-root.md` 报告
  `19:2: error [local-target.outside-root]: target "../../../../etc/hosts" resolves outside the workspace root`

[返回检查规则演示索引](00-index.md)
