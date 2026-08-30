---
title: 引用不存在的本地文件
description: 演示检查器如何定位不存在的本地图片或文件引用。
tags:
  - 文档检查
  - 本地引用
create_date: 2026-08-30
update_date: 2026-08-30
checker: local-target.missing
---

# 引用不存在的本地文件

检查规则：`local-target.missing`

下面这张图片引用了一个不存在的路径：

![缺失的拓扑图](assets/missing-topology.png)

预期表现：

- `m2h check docs/demos/checkers/02-local-target-missing.md` 报告
  `18:3: error [local-target.missing]: target "assets/missing-topology.png" does not exist`
- 图片写有 alt 文本，因此不会同时触发 `image.alt-empty`

[返回检查规则演示索引](00-index.md)
