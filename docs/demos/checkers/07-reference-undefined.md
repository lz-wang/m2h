---
title: 引用了不存在的 reference 定义
---

# 引用了不存在的 reference 定义

下面这个 reference-style 链接写了 `[安装指南][install]`，但全文没有
`[install]: …` 定义，渲染结果是一段字面文本而不是链接：

[安装指南][install]

预期表现：

- `m2h check docs/demos/checkers/07-reference-undefined.md` 报告
  `10:1: error [reference.undefined]: reference label "install" is not defined`
- 裸 `[标签]`（shortcut 形式）没有定义时不报——普通文档里方括号短语太常见
