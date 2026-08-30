---
title: 写反的链接语法
---

# 写反的链接语法

圆括号和方括号写反了——正确写法是 `[OpenAI](https://openai.com)`：

(OpenAI)[https://openai.com]

预期表现：

- `m2h check docs/demos/checkers/12-link-reversed.md` 报告
  `9:1: error [link.reversed]: looks like reversed Markdown link syntax; use [OpenAI](https://openai.com)`
- 本规则只认 destination 明显像 URL/路径的高置信度场景：
  `f(x)[0]`、`array[index]` 这类普通文本不报
