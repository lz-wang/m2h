---
title: 元数据标题
description: 演示 frontmatter.title-mismatch 的判定与边界。
tags: [文档检查]
create_date: 2026-09-10
update_date: 2026-09-10
---

# 正文标题

检查规则：`frontmatter.title-mismatch`

当非空、可用的 Frontmatter title 与正文首个 H1 都存在时才比较；缺少任一项不告警。
比较复用解析后的标题文本，忽略首尾与连续空白，不区分 ATX/Setext 写法；大小写仍须一致。
多个 H1 仅比较第一个，其余交给 document.multiple-h1。
本例故意使用不同标题，诊断定位到 title 字段：

`2:1: warning [frontmatter.title-mismatch]: frontmatter title "元数据标题" does not match first H1 "正文标题"`

[返回检查规则演示索引](00-index.md)
