---
title: 没有关闭的 HTML 注释
---

# 没有关闭的 HTML 注释

下面的注释只有开头 `<!--`、没有结尾 `-->`。从那一行起，文档其余内容全部
渲染成注释正文——在浏览器里"消失"。这也是本演示把注释放在最后一行的原因。

预期表现：

- `m2h check docs/demos/checkers/11-html-comment-unclosed.md` 报告
  `16:1: error [html.comment-unclosed]: unclosed HTML comment; everything after it renders as comment content`
- 诊断定位在注释开头所在行；Frontmatter 值里的 `<!--` 是 YAML 数据，不检查

<!-- 遗忘的注释，后面的内容都被吞掉了
