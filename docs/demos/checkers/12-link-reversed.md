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
- HTML block/comment 与 inline HTML tag token 中的形状不参与；inline tag
  之间仍是 Markdown 文本，已接受 inline link 的 destination/title 保持字面语法
