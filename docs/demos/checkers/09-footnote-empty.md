---
title: 空的脚注定义
---

# 空的脚注定义

下面的脚注被引用了，但定义冒号后面什么都没有，页脚会渲染一个空脚注：

正文引用[^empty]。

[^empty]:

预期表现：

- `m2h check docs/demos/checkers/09-footnote-empty.md` 报告
  `11:1: error [footnote.empty]: footnote [^empty] has no content`
- 冒号后换行、以缩进续行书写多行内容是合法的，不触发本规则
