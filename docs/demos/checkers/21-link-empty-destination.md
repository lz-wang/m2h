---
title: 空的链接目标
description: 演示 Markdown 链接 destination 为空时的警告。
tags:
  - 文档检查
  - 链接
create_date: 2026-08-30
update_date: 2026-08-30
checker: link.empty-destination
---

# 空的链接目标

检查规则：`link.empty-destination`

下面这组链接与图片的 destination 是空的，点上去哪儿也去不了：

[空目标链接]() 与 ![](assets/logo.png) 的对比：图片路径合法，链接为空。

预期表现：

- `m2h check docs/demos/checkers/21-link-empty-destination.md` 报告
  `18:2: warning [link.empty-destination]: link destination is empty`
- 同一行的 `![](assets/logo.png)` 还会触发
  `18:29: warning [image.alt-empty]`——两个规则各自独立报告
- 注意 `![](assets/logo.png)` 同时触发 `image.alt-empty`（空 alt），
  两个规则各自独立报告

[返回检查规则演示索引](00-index.md)
