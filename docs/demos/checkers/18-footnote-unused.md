---
title: 从未被引用的脚注
---

# 从未被引用的脚注

下面的脚注有内容，但正文从头到尾没有出现过 `[^orphan]` 标记，读者永远走
不到这里：

[^orphan]: 一条有内容却从未被引用的脚注。

预期表现：

- `m2h check docs/demos/checkers/18-footnote-unused.md` 报告
  `10:1: warning [footnote.unused]: footnote [^orphan] is never referenced`
- 定义有内容，因此不触发 `footnote.empty`
