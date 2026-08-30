---
title: 空的脚注定义
description: 演示脚注定义没有内容时的错误诊断。
tags:
  - 文档检查
  - 脚注
create_date: 2026-08-30
update_date: 2026-08-30
checker: footnote.empty
---

# 空的脚注定义

检查规则：`footnote.empty`

下面的脚注被引用了，但定义冒号后面什么都没有，页脚会渲染一个空脚注：

正文引用[^empty]。

[^empty]:

预期表现：

- `m2h check docs/demos/checkers/09-footnote-empty.md` 报告
  `20:1: error [footnote.empty]: footnote [^empty] has no content`
- 冒号后换行、以缩进续行书写多行内容是合法的，不触发本规则

[返回检查规则演示索引](00-index.md)
