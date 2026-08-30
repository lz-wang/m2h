---
title: 引用了不存在的 reference 定义
description: 演示未定义 reference-style 链接标签的诊断。
tags:
  - 文档检查
  - 引用链接
create_date: 2026-08-30
update_date: 2026-08-30
checker: reference.undefined
---

# 引用了不存在的 reference 定义

检查规则：`reference.undefined`

下面这个 reference-style 链接写了 `[安装指南][install]`，但全文没有
`[install]: …` 定义，渲染结果是一段字面文本而不是链接：

[安装指南][install]

预期表现：

- `m2h check docs/demos/checkers/07-reference-undefined.md` 报告
  `19:1: error [reference.undefined]: reference label "install" is not defined`
- 裸 `[标签]`（shortcut 形式）没有定义时不报——普通文档里方括号短语太常见
- HTML block/comment 与 inline HTML tag token 中的括号不参与；但
  `<code>[示例][missing]</code>` 的 tag 之间仍是普通 Markdown，会参与检查
- 已被解析器接受的 inline link destination/title 不做嵌套 Markdown 解析，
  其中的 `[示例][missing]` 不会占用真实 undefined reference 的诊断位置

[返回检查规则演示索引](00-index.md)
