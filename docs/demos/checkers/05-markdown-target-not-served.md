---
title: 目标不在当前服务范围
description: 演示单文件模式下不可访问的相邻 Markdown 文档链接。
tags:
  - 文档检查
  - 文档范围
create_date: 2026-08-30
update_date: 2026-08-30
checker: markdown-target.not-served
---

# 目标不在当前服务范围

检查规则：`markdown-target.not-served`

下面的链接指向 `assets/target.md`——文件确实存在且完全合法，但在**单文件
模式**下工作区只服务这一篇文档，目标永远点不开：

[相邻文档](assets/target.md)

预期表现：

- `m2h check docs/demos/checkers/05-markdown-target-not-served.md` 报告
  `19:2: error [markdown-target.not-served]: Markdown target "assets/target.md" exists but is not available in single-file mode`
- 目录模式 `m2h check docs/demos/checkers` 下同一链接合法，不产生诊断

[返回检查规则演示索引](00-index.md)
