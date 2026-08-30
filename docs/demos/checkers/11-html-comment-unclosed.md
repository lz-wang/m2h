---
title: 没有关闭的 HTML 注释
description: 演示未闭合 HTML 注释吞没后续文档内容时的诊断。
tags:
  - 文档检查
  - HTML
create_date: 2026-08-30
update_date: 2026-08-30
checker: html.comment-unclosed
---

# 没有关闭的 HTML 注释

检查规则：`html.comment-unclosed`

下面的注释只有开头 `<!--`、没有结尾 `-->`。从那一行起，文档其余内容全部
渲染成注释正文——在浏览器里"消失"。这也是本演示把注释放在最后一行的原因。

预期表现：

- `m2h check docs/demos/checkers/11-html-comment-unclosed.md` 报告
  `27:1: error [html.comment-unclosed]: unclosed HTML comment; everything after it renders as comment content`
- 诊断定位在注释开头所在行；Frontmatter 值里的 `<!--` 是 YAML 数据，不检查

[返回检查规则演示索引](00-index.md)

<!-- 遗忘的注释，后面的内容都被吞掉了
