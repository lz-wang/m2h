---
title: 没有正文的章节
---

# 标题

## 空章节

## 下一节

"空章节"和下一节之间没有任何渲染内容，本节自身也没有正文。

预期表现：

- 默认不检查：`m2h check docs/demos/checkers/22-section-empty.md` 无诊断
- `m2h check docs/demos/checkers/22-section-empty.md --enable section.empty`
  报告 `5:1` 与 `7:1` 两条 `warning [section.empty]: section … has no content`
  （H1 只承载子标题同样算空）
- 父章节只做结构、正文全在子章节是常见合法写法，因此本规则默认关闭
