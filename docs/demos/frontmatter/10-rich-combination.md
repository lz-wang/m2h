---
title: 综合示例：发布说明
author: lzwang
category: release-notes
create_date: 2026-08-20T09:30:00+08:00
update_date: 2026-08-28 21:12
tags:
  - 发布
  - frontmatter
  - 演示
review:
  reviewer: alice
  approved: true
---

# 综合示例：发布说明

接近真实使用的组合：自定义字段、双时间别名、块序列 tags 与嵌套映射。

预期表现：

- 摘要显示 **created 2026-08-20**、**updated 2026-08-28** 与三个标签 `发布 · frontmatter · 演示`。
- 表格列出全部 7 个条目：自定义键 `author`、`category` 原样展示，嵌套映射 `review` 重排为可读 YAML。
- 文档标题取 frontmatter `title`，覆盖 H1 文本。
