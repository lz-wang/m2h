---
title: 来自 Frontmatter 的标题
create_date: 2026-08-28
---

# 正文里的 H1 标题

frontmatter 的标量 `title` 与正文 H1 同时存在且内容不同。

预期表现：

- 侧边栏与工具栏标题均为 **来自 Frontmatter 的标题**，frontmatter 优先级最高。
- 正文 H1 原样渲染在文档里，不参与标题推导。
