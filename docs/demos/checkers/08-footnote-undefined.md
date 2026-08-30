---
title: 脚注标记没有定义
description: 演示正文脚注标记没有对应定义时的诊断。
tags:
  - 文档检查
  - 脚注
create_date: 2026-08-30
update_date: 2026-08-30
checker: footnote.undefined
---

# 脚注标记没有定义

检查规则：`footnote.undefined`

正文里的脚注标记没有对应的定义。

参见脚注[^missing]。

预期表现：

- `m2h check docs/demos/checkers/08-footnote-undefined.md` 报告
  `18:13: error [footnote.undefined]: footnote [^missing] is not defined`
- 脚注标签按渲染器同一逐字节语义比较，`[^Missing]` 与 `[^missing]` 不互通
- HTML block/comment 与 inline HTML tag token 中的标记不参与；inline tag
  之间的文本仍按 Markdown 解析，inline link destination/title 则保持链接字面语法

[返回检查规则演示索引](00-index.md)
