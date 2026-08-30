---
title: 一个文档多个 H1
---

# 第一个 H1 是文档标题

侧边栏与文档标题只使用第一个 H1，第二个及之后的 H1 会让目录结构出现两个
平级的"文档级"节点：

# 第二个 H1 是多余的

预期表现：

- `m2h check docs/demos/checkers/14-document-multiple-h1.md` 报告
  `10:1: warning [document.multiple-h1]: document contains 2 H1 headings`
- 诊断定位在**第二个** H1（第一个多余的）所在行
