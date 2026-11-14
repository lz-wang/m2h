---
title: HTML 块后缺少空行
description: 演示 Type 6/7 HTML 块结束后直接跟随正文时的警告。
tags:
  - 文档检查
  - HTML
create_date: 2026-11-14
update_date: 2026-11-14
checker: html.block-blank-after
---

# HTML 块后缺少空行

检查规则：`html.block-blank-after`

CommonMark 的 Type 6/7 HTML 块以空行结束，而不是遇到关闭标签就结束。
下面的 `</div>` 之后没有空行，紧随其后的 `**加粗正文**` 仍属于同一个
原始 HTML 块，渲染时原样输出而不是按 Markdown 解析——加粗失效。
在 `</div>` 与正文之间补一个空行即可恢复 Markdown 解析。

预期表现：

- `m2h check docs/demos/checkers/29-html-block-blank-after.md` 报告
  `37:1: warning [html.block-blank-after]: HTML block is followed by non-blank content without a separating blank line`
- 诊断定位在受影响的正文行首列，即需要插入空行的位置
- 只检查 Type 6/7 HTML 块；`<script>`（Type 1）与 HTML 注释（Type 2）
  有自己的结束条件，不在本规则范围
- 同一块内连续的顶层 HTML 元素（如 `<div></div>` 后跟 `<div>`）与
  void 元素占行的场景才参与判定；标签嵌套不匹配、容器前缀无法恢复等
  低置信度情况一律跳过，宁可漏报不误报

[返回检查规则演示索引](00-index.md)

<div class="notice">
  提示内容
</div>
**加粗正文**
