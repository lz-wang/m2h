---
title: 引用不存在的本地文件
---

# 引用不存在的本地文件

下面这张图片引用了一个不存在的路径：

![缺失的拓扑图](assets/missing-topology.png)

预期表现：

- `m2h check docs/demos/checkers/02-local-target-missing.md` 报告
  `9:3: error [local-target.missing]: target "assets/missing-topology.png" does not exist`
- 图片写有 alt 文本，因此不会同时触发 `image.alt-empty`
