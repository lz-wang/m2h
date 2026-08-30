---
title: 未闭合的序列
tags: [演示, check
---

# 未闭合的 YAML

上面的 frontmatter 里 `tags` 的方括号没有闭合，YAML 无法解析，整个文档不再
可渲染——WebUI 打开本文件会返回 422 错误页，文件列表以文件名降级显示。

预期表现：

- `m2h check docs/demos/checkers/01-frontmatter-invalid.md` 报告
  `1:1: error [frontmatter.invalid]: frontmatter is not valid YAML: …`
- frontmatter 解析失败的文档不做引用检查，不会级联产生其他诊断
